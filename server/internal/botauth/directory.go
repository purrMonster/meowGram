// Package botauth implements request-scoped LLDAP authentication. It never
// issues tokens or treats a persisted principal as an authorization grant.
package botauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
)

var (
	ErrCredentials = errors.New("invalid credentials")
	ErrForbidden   = errors.New("bot group required")
	ErrUnavailable = errors.New("authentication unavailable")
)

type Identity struct{ Namespace, ExternalID, DisplayName string }

// Options contains deployment configuration only. Reader passwords are loaded
// from a host-local file and never accepted through a URL or a logging field.
type Options struct {
	URL, BaseDN, ReaderDN, ReaderPasswordFile, AllowedGroup, Namespace, CAFile string
	Timeout                                                                    time.Duration
	MaxConcurrent                                                              int
}

type connection interface {
	Bind(string, string) error
	Search(*ldap.SearchRequest) (*ldap.SearchResult, error)
	Close() error
}

type Directory struct {
	options  Options
	password string
	group    *ldap.DN
	slots    chan struct{}
	dial     func(context.Context) (connection, error)
}

func NewDirectory(o Options) (*Directory, error) {
	u, err := url.Parse(o.URL)
	if err != nil || u.Scheme != "ldaps" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("BOT_LDAP_URL must be an LDAPS origin without credentials or path")
	}
	if o.Timeout <= 0 || o.Timeout > 30*time.Second || o.MaxConcurrent < 1 || o.MaxConcurrent > 128 {
		return nil, errors.New("bot LDAP timeout or concurrency outside supported bounds")
	}
	if strings.TrimSpace(o.Namespace) == "" || utf8.RuneCountInString(o.Namespace) > 255 {
		return nil, errors.New("bot directory namespace is required and limited to 255 characters")
	}
	for _, value := range []string{o.BaseDN, o.ReaderDN, o.AllowedGroup} {
		dn, e := ldap.ParseDN(value)
		if e != nil || len(dn.RDNs) == 0 {
			return nil, errors.New("bot LDAP base, reader and group must be valid nonempty DNs")
		}
	}
	password, err := os.ReadFile(o.ReaderPasswordFile)
	if err != nil || len(password) == 0 || len(password) > 4096 {
		return nil, errors.New("bot LDAP reader password file must contain 1 to 4096 bytes")
	}
	// Only remove a conventional final line ending; spaces are password bytes.
	secret := strings.TrimSuffix(strings.TrimSuffix(string(password), "\n"), "\r")
	if secret == "" {
		return nil, errors.New("bot LDAP reader password must not be empty")
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.New("system certificate roots unavailable")
	}
	if o.CAFile != "" {
		pem, e := os.ReadFile(o.CAFile)
		if e != nil || !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("bot LDAP CA file is invalid")
		}
	}
	group, _ := ldap.ParseDN(o.AllowedGroup)
	d := &Directory{options: o, password: secret, group: group, slots: make(chan struct{}, o.MaxConcurrent)}
	address := u.Host
	if u.Port() == "" {
		address = net.JoinHostPort(u.Hostname(), "636")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname(), RootCAs: roots}
	d.dial = func(ctx context.Context) (connection, error) {
		raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, ErrUnavailable
		}
		secure := tls.Client(raw, tlsConfig)
		if err = secure.HandshakeContext(ctx); err != nil {
			raw.Close()
			return nil, ErrUnavailable
		}
		deadline, ok := ctx.Deadline()
		if !ok {
			secure.Close()
			return nil, ErrUnavailable
		}
		if err = secure.SetDeadline(deadline); err != nil {
			secure.Close()
			return nil, ErrUnavailable
		}
		conn := ldap.NewConn(secure, true)
		conn.SetTimeout(time.Until(deadline))
		conn.Start()
		// Close the socket, not LDAP's potentially waiting shutdown, on cancellation.
		stop := context.AfterFunc(ctx, func() { raw.Close() })
		return &scopedConnection{Conn: conn, stop: stop}, nil
	}
	return d, nil
}

type scopedConnection struct {
	*ldap.Conn
	stop func() bool
}

func (c *scopedConnection) Close() error { c.stop(); return c.Conn.Close() }

func validCredentials(username, password string) bool {
	return username != "" && len(username) <= 256 && password != "" && len(password) <= 4096 &&
		utf8.ValidString(username) && !strings.ContainsAny(username, "\x00\r\n")
}

// Authenticate performs fresh reader lookup, isolated password bind and fresh
// membership lookup under one total deadline. Neither grants nor passwords cache.
func (d *Directory) Authenticate(ctx context.Context, username, password string) (Identity, error) {
	if !validCredentials(username, password) {
		return Identity{}, ErrCredentials
	}
	select {
	case d.slots <- struct{}{}:
		defer func() { <-d.slots }()
	default:
		return Identity{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, d.options.Timeout)
	defer cancel()
	reader, err := d.dial(ctx)
	if err != nil {
		return Identity{}, ErrUnavailable
	}
	defer reader.Close()
	if err = reader.Bind(d.options.ReaderDN, d.password); err != nil {
		return Identity{}, ErrUnavailable
	}
	before, err := d.lookup(reader, username)
	if err != nil {
		return Identity{}, err
	}
	beforeID, err := entryID(before)
	if err != nil {
		return Identity{}, err
	}
	beforeDN, err := ldap.ParseDN(before.DN)
	if err != nil || len(beforeDN.RDNs) == 0 {
		return Identity{}, ErrUnavailable
	}
	user, err := d.dial(ctx)
	if err != nil {
		return Identity{}, ErrUnavailable
	}
	defer user.Close()
	if err = user.Bind(before.DN, password); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return Identity{}, ErrCredentials
		}
		return Identity{}, ErrUnavailable
	}
	after, err := d.lookup(reader, username)
	if err != nil {
		return Identity{}, err
	}
	afterID, err := entryID(after)
	if err != nil {
		return Identity{}, err
	}
	afterDN, err := ldap.ParseDN(after.DN)
	if err != nil || len(afterDN.RDNs) == 0 {
		return Identity{}, ErrUnavailable
	}
	if beforeID != afterID || !beforeDN.EqualFold(afterDN) {
		return Identity{}, ErrCredentials
	}
	allowed := false
	for _, value := range after.GetAttributeValues("memberOf") {
		dn, e := ldap.ParseDN(value)
		if e != nil {
			return Identity{}, ErrUnavailable
		}
		if d.group.EqualFold(dn) {
			allowed = true
		}
	}
	if ctx.Err() != nil {
		return Identity{}, ErrUnavailable
	}
	if !allowed {
		return Identity{}, ErrForbidden
	}
	name := after.GetAttributeValue("cn")
	if name == "" {
		name = username
	}
	return Identity{Namespace: d.options.Namespace, ExternalID: afterID, DisplayName: name}, nil
}

func (d *Directory) lookup(c connection, username string) (*ldap.Entry, error) {
	result, err := c.Search(ldap.NewSearchRequest(d.options.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, 0, false,
		"(uid="+ldap.EscapeFilter(username)+")", []string{"entryUUID", "cn", "memberOf"}, nil))
	if err != nil || result == nil || len(result.Referrals) > 0 {
		return nil, ErrUnavailable
	}
	if len(result.Entries) == 0 {
		return nil, ErrCredentials
	}
	if len(result.Entries) != 1 || result.Entries[0] == nil {
		return nil, ErrUnavailable
	}
	return result.Entries[0], nil
}

func entryID(e *ldap.Entry) (string, error) {
	values := e.GetAttributeValues("entryUUID")
	if len(values) != 1 {
		return "", ErrUnavailable
	}
	id, err := uuid.Parse(values[0])
	if err != nil || id == uuid.Nil {
		return "", ErrUnavailable
	}
	return id.String(), nil
}

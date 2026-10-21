package botauth

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
)

func TestTransportDeadlineAndConfiguration(t *testing.T) {
	secretFile := filepath.Join(t.TempDir(), "reader")
	if err := os.WriteFile(secretFile, []byte("disposable-test-value"), 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, e := listener.Accept()
		if e == nil {
			accepted <- conn
		}
	}()
	o := Options{URL: "ldaps://" + listener.Addr().String(), BaseDN: "ou=people,dc=example,dc=com", ReaderDN: "uid=reader,dc=example,dc=com", ReaderPasswordFile: secretFile, AllowedGroup: groupDN, Namespace: "test", Timeout: 100 * time.Millisecond, MaxConcurrent: 1}
	d, err := NewDirectory(o)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err = d.Authenticate(context.Background(), "bot", "test-value"); err != ErrUnavailable {
		t.Fatal("stalled TLS did not fail closed")
	}
	if time.Since(start) > time.Second {
		t.Fatal("handshake exceeded deadline")
	}
	select {
	case conn := <-accepted:
		conn.Close()
	case <-time.After(time.Second):
		t.Fatal("transport not exercised")
	}
	for _, bad := range []string{"ldap://localhost:389", "ldaps://user:password@localhost", "ldaps://localhost/path", "ldaps://localhost?query=value"} {
		o.URL = bad
		if _, err = NewDirectory(o); err == nil {
			t.Fatal("unsafe URL accepted")
		}
	}
}

const testID = "11111111-1111-4111-8111-111111111111"
const groupDN = "cn=bots,ou=groups,dc=example,dc=com"

type fakeConn struct {
	bind   func(string, string) error
	search func(*ldap.SearchRequest) (*ldap.SearchResult, error)
}

func (f *fakeConn) Bind(u, p string) error {
	if f.bind != nil {
		return f.bind(u, p)
	}
	return nil
}
func (f *fakeConn) Search(r *ldap.SearchRequest) (*ldap.SearchResult, error) { return f.search(r) }
func (f *fakeConn) Close() error                                             { return nil }
func entry(id string, member bool) *ldap.Entry {
	attrs := map[string][]string{"entryUUID": {id}, "uid": {"bot"}, "cn": {"Backup"}}
	if member {
		attrs["memberOf"] = []string{groupDN}
	}
	return ldap.NewEntry("uid=bot,ou=people,dc=example,dc=com", attrs)
}
func fakeDirectory(reader, user connection) *Directory {
	group, _ := ldap.ParseDN(groupDN)
	d := &Directory{options: Options{Namespace: "test", Timeout: time.Second}, group: group, slots: make(chan struct{}, 1)}
	calls := 0
	d.dial = func(context.Context) (connection, error) {
		calls++
		if calls%2 == 1 {
			return reader, nil
		}
		return user, nil
	}
	return d
}

func TestDirectoryFailuresAndRevalidation(t *testing.T) {
	cases := []struct {
		name                     string
		first, last              []*ldap.Entry
		searchErr, bindErr, want error
	}{
		{name: "member", first: []*ldap.Entry{entry(testID, true)}, last: []*ldap.Entry{entry(testID, true)}},
		{name: "unknown", want: ErrCredentials},
		{name: "ambiguous", first: []*ldap.Entry{entry(testID, true), entry(testID, true)}, want: ErrUnavailable},
		{name: "password", first: []*ldap.Entry{entry(testID, true)}, bindErr: ldap.NewError(49, errors.New("sensitive provider detail")), want: ErrCredentials},
		{name: "bind outage", first: []*ldap.Entry{entry(testID, true)}, bindErr: errors.New("sensitive detail"), want: ErrUnavailable},
		{name: "lookup outage", searchErr: errors.New("sensitive detail"), want: ErrUnavailable},
		{name: "revoked", first: []*ldap.Entry{entry(testID, true)}, last: []*ldap.Entry{entry(testID, false)}, want: ErrForbidden},
		{name: "deleted", first: []*ldap.Entry{entry(testID, true)}, want: ErrCredentials},
		{name: "recreated", first: []*ldap.Entry{entry(testID, true)}, last: []*ldap.Entry{entry("22222222-2222-4222-8222-222222222222", true)}, want: ErrCredentials},
		{name: "missing UUID", first: []*ldap.Entry{entry("", true)}, last: []*ldap.Entry{entry("", true)}, want: ErrUnavailable},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			searches := 0
			reader := &fakeConn{search: func(r *ldap.SearchRequest) (*ldap.SearchResult, error) {
				if r.Filter != "(uid=bot)" || r.SizeLimit != 2 || r.DerefAliases != ldap.NeverDerefAliases {
					t.Fatal("unsafe search")
				}
				searches++
				entries := tt.first
				if searches > 1 {
					entries = tt.last
				}
				return &ldap.SearchResult{Entries: entries}, tt.searchErr
			}}
			user := &fakeConn{bind: func(dn, p string) error {
				if !strings.HasPrefix(dn, "uid=bot,") {
					t.Fatal("did not bind resolved DN")
				}
				return tt.bindErr
			}}
			d := fakeDirectory(reader, user)
			got, err := d.Authenticate(context.Background(), "bot", "generated-test-value")
			if !errors.Is(err, tt.want) {
				t.Fatalf("outcome %v, expected %v", err, tt.want)
			}
			if err == nil && (got.ExternalID != testID || got.Namespace != "test" || got.Username != "bot" || searches != 2) {
				t.Fatal("invalid authenticated identity")
			}
		})
	}
}

func TestEscapingAndNoEmptyBind(t *testing.T) {
	calls := 0
	reader := &fakeConn{search: func(r *ldap.SearchRequest) (*ldap.SearchResult, error) {
		calls++
		if r.Filter != `(uid=\2a\29\28uid=\2a\29)` {
			t.Fatalf("unescaped filter %s", r.Filter)
		}
		return &ldap.SearchResult{}, nil
	}}
	d := fakeDirectory(reader, &fakeConn{})
	for _, pair := range [][2]string{{"", "x"}, {"u", ""}, {"u\x00", "p"}, {strings.Repeat("u", 257), "p"}} {
		_, err := d.Authenticate(context.Background(), pair[0], pair[1])
		if err != ErrCredentials {
			t.Fatal("invalid input accepted")
		}
	}
	if calls != 0 {
		t.Fatal("invalid input contacted directory")
	}
	_, _ = d.Authenticate(context.Background(), "*)(uid=*)", "x")
	if calls != 1 {
		t.Fatal("search not exercised")
	}
}

func TestConcurrencyAndCancellation(t *testing.T) {
	group, _ := ldap.ParseDN(groupDN)
	entered := make(chan struct{})
	d := &Directory{options: Options{Timeout: time.Second}, group: group, slots: make(chan struct{}, 1)}
	d.dial = func(ctx context.Context) (connection, error) { close(entered); <-ctx.Done(); return nil, ctx.Err() }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := d.Authenticate(ctx, "bot", "x"); done <- err }()
	<-entered
	if _, err := d.Authenticate(context.Background(), "bot", "x"); err != ErrUnavailable {
		t.Fatal("concurrency bound ignored")
	}
	cancel()
	select {
	case err := <-done:
		if err != ErrUnavailable {
			t.Fatal("cancellation not sanitized")
		}
	case <-time.After(time.Second):
		t.Fatal("request did not cancel")
	}
	if len(d.slots) != 0 {
		t.Fatal("slot leaked")
	}
}

func TestDirectoryUsesVerifiedUIDNotDisplayName(t *testing.T) {
	for _, display := range []string{"First display", "Renamed display"} {
		reader := &fakeConn{search: func(r *ldap.SearchRequest) (*ldap.SearchResult, error) {
			if r.Filter != "(uid=BOT)" {
				t.Fatal("unexpected lookup")
			}
			foundUID := false
			for _, attr := range r.Attributes {
				if attr == "uid" {
					foundUID = true
				}
			}
			if !foundUID {
				t.Fatal("uid not requested")
			}
			return &ldap.SearchResult{Entries: []*ldap.Entry{ldap.NewEntry("uid=bot,ou=people,dc=example,dc=com", map[string][]string{
				"entryUUID": {testID}, "uid": {"bot"}, "cn": {display}, "memberOf": {groupDN},
			})}}, nil
		}}
		got, err := fakeDirectory(reader, &fakeConn{}).Authenticate(context.Background(), "BOT", "test-value")
		if err != nil || got.Username != "bot" || got.ExternalID != testID {
			t.Fatalf("canonical uid lost: %v", err)
		}
	}
}

func TestDirectoryRejectsInvalidOrChangingUID(t *testing.T) {
	for _, test := range []struct {
		name          string
		before, after []string
		want          error
	}{
		{"missing", nil, nil, ErrUnavailable},
		{"empty", []string{""}, []string{""}, ErrUnavailable},
		{"multiple", []string{"bot", "other"}, nil, ErrUnavailable},
		{"oversized", []string{strings.Repeat("x", 101)}, nil, ErrUnavailable},
		{"space", []string{" bot"}, nil, ErrUnavailable},
		{"control", []string{"bot\n"}, nil, ErrUnavailable},
		{"changed during bind", []string{"bot"}, []string{"other"}, ErrCredentials},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			reader := &fakeConn{search: func(*ldap.SearchRequest) (*ldap.SearchResult, error) {
				values := test.before
				calls++
				if calls > 1 {
					values = test.after
				}
				return &ldap.SearchResult{Entries: []*ldap.Entry{ldap.NewEntry("uid=bot,ou=people,dc=example,dc=com", map[string][]string{
					"entryUUID": {testID}, "uid": values, "cn": {"fallback forbidden"}, "memberOf": {groupDN},
				})}}, nil
			}}
			_, err := fakeDirectory(reader, &fakeConn{}).Authenticate(context.Background(), "bot", "test-value")
			if !errors.Is(err, test.want) {
				t.Fatalf("got %v want %v", err, test.want)
			}
		})
	}
}

package botauth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Runs only in the disposable pinned-provider image; no live configuration is read.
func TestLLDAP(t *testing.T) {
	if os.Getenv("MEOWGRAM_DISPOSABLE_LLDAP") != "1" {
		t.Skip("use verify-bot-auth script for pinned LLDAP integration")
	}
	root := t.TempDir()
	write := func(name string, data []byte) string {
		path := filepath.Join(root, name)
		if os.WriteFile(path, data, 0600) != nil {
			t.Fatal("fixture file failed")
		}
		return path
	}
	secret := func() string { return uuid.NewString() + uuid.NewString() }
	admin, readerPassword, botPassword := secret(), secret(), secret()
	port := func() int {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal("fixture port unavailable")
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	}
	ldapPort, httpPort, tlsPort := port(), port(), port()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal("fixture key failed")
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	cert, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal("fixture cert failed")
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal("fixture key encoding failed")
	}
	ca := write("cert.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}))
	keyPath := write("key.pem", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}))
	base := "dc=example,dc=com"
	config := fmt.Sprintf("ldap_host = \"127.0.0.1\"\nhttp_host = \"127.0.0.1\"\nldap_port = %d\nhttp_port = %d\nhttp_url = \"http://localhost:%d\"\nldap_base_dn = %q\nldap_user_dn = \"admin\"\nldap_user_pass = %q\njwt_secret = %q\nkey_seed = %q\ndatabase_url = %q\n[ldaps_options]\nenabled = true\nport = %d\ncert_file = %q\nkey_file = %q\n", ldapPort, httpPort, httpPort, base, admin, secret(), secret(), "sqlite://"+filepath.Join(root, "users.db")+"?mode=rwc", tlsPort, ca, keyPath)
	cmd := exec.Command("/app/lldap", "run", "--config-file", write("lldap.toml", []byte(config)))
	if cmd.Start() != nil {
		t.Fatal("fixture provider failed to start")
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	client := &http.Client{Timeout: 2 * time.Second}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	request := func(path string, body any, token string) (map[string]any, bool) {
		data, e := json.Marshal(body)
		if e != nil {
			return nil, false
		}
		req, e := http.NewRequest("POST", endpoint+path, bytes.NewReader(data))
		if e != nil {
			return nil, false
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, e := client.Do(req)
		if e != nil {
			return nil, false
		}
		defer response.Body.Close()
		var result map[string]any
		if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result) != nil {
			return nil, false
		}
		return result, true
	}
	token := ""
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		result, ok := request("/auth/simple/login", map[string]string{"username": "admin", "password": admin}, "")
		if ok {
			token, _ = result["token"].(string)
			if token != "" {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if token == "" {
		t.Fatal("fixture provider not ready")
	}
	graphql := func(query string, variables map[string]any) map[string]any {
		result, ok := request("/api/graphql", map[string]any{"query": query, "variables": variables}, token)
		if !ok || result["errors"] != nil {
			t.Fatal("fixture provisioning failed (provider details suppressed)")
		}
		data, ok := result["data"].(map[string]any)
		if !ok {
			t.Fatal("invalid fixture response")
		}
		return data
	}
	create := func(user string) {
		graphql(`mutation($u:CreateUserInput!){createUser(user:$u){uuid}}`, map[string]any{"u": map[string]string{"id": user, "email": user + "@example.com", "displayName": user}})
	}
	membership := func(user string, group int, add bool) {
		op := "addUserToGroup"
		if !add {
			op = "removeUserFromGroup"
		}
		graphql(fmt.Sprintf(`mutation($u:String!,$g:Int!){%s(userId:$u,groupId:$g){ok}}`, op), map[string]any{"u": user, "g": group})
	}
	setPassword := func(user, password string) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, "/app/lldap_set_password", "--base-url", endpoint, "--token", token, "--username", user)
		c.Env = append(os.Environ(), "LLDAP_USER_PASSWORD="+password)
		if c.Run() != nil {
			t.Fatal("fixture password setup failed (details suppressed)")
		}
	}
	create("reader")
	create("bot")
	groups := graphql(`{groups{id displayName}}`, nil)["groups"].([]any)
	readonly := -1
	for _, raw := range groups {
		g := raw.(map[string]any)
		if g["displayName"] == "lldap_strict_readonly" {
			readonly = int(g["id"].(float64))
		}
	}
	if readonly < 0 {
		t.Fatal("strict readonly group absent")
	}
	membership("reader", readonly, true)
	group := int(graphql(`mutation{createGroup(name:"bots"){id}}`, nil)["createGroup"].(map[string]any)["id"].(float64))
	membership("bot", group, true)
	setPassword("reader", readerPassword)
	setPassword("bot", botPassword)
	options := Options{URL: fmt.Sprintf("ldaps://localhost:%d", tlsPort), BaseDN: "ou=people," + base, ReaderDN: "uid=reader,ou=people," + base, ReaderPasswordFile: write("reader-secret", []byte(readerPassword)), AllowedGroup: "cn=bots,ou=groups," + base, Namespace: "disposable", CAFile: ca, Timeout: 3 * time.Second, MaxConcurrent: 2}
	directory, err := NewDirectory(options)
	if err != nil {
		t.Fatal("directory initialization failed")
	}
	check := func(user, password string, want error) Identity {
		t.Helper()
		identity, e := directory.Authenticate(context.Background(), user, password)
		if !errors.Is(e, want) {
			t.Fatalf("authentication outcome %v, expected %v", e, want)
		}
		return identity
	}
	original := check("bot", botPassword, nil)
	check("bot", secret(), ErrCredentials)
	check("unknown", secret(), ErrCredentials)
	graphql(`mutation($u:UpdateUserInput!){updateUser(user:$u){ok}}`, map[string]any{"u": map[string]string{"id": "bot", "displayName": "Renamed bot"}})
	renamed := check("bot", botPassword, nil)
	if original.ExternalID != renamed.ExternalID || original.Username != "bot" || renamed.Username != "bot" {
		t.Fatal("display-name rename changed login username or identity")
	}
	membership("bot", group, false)
	check("bot", botPassword, ErrForbidden)
	membership("bot", group, true)
	check("bot", botPassword, nil)
	rotated := secret()
	setPassword("bot", rotated)
	check("bot", botPassword, ErrCredentials)
	check("bot", rotated, nil)
	graphql(`mutation{deleteUser(userId:"bot"){ok}}`, nil)
	check("bot", rotated, ErrCredentials)
	create("bot")
	setPassword("bot", rotated)
	check("bot", rotated, ErrForbidden)
	membership("bot", group, true)
	recreated := check("bot", rotated, nil)
	if recreated.ExternalID == original.ExternalID || recreated.Username != "bot" {
		t.Fatal("recreation reused identity")
	}
	options.CAFile = ""
	untrusted, e := NewDirectory(options)
	if e != nil {
		t.Fatal("system trust init failed")
	}
	if _, e = untrusted.Authenticate(context.Background(), "bot", rotated); e != ErrUnavailable {
		t.Fatal("untrusted TLS accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = directory.Authenticate(ctx, "bot", rotated); e != ErrUnavailable {
		t.Fatal("cancelled request accepted")
	}
	_ = cmd.Process.Kill()
	check("bot", rotated, ErrUnavailable)
}

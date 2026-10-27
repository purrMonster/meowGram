//go:build r21_contracts

package integration

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"encoding/pem"
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

	"crypto/rsa"
	"encoding/base64"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
)

// Runs only in the disposable pinned-provider image; no live configuration is read.
func TestR21HTTPContract(t *testing.T) {
	if os.Getenv("MEOWGRAM_DISPOSABLE_R21_CONTRACT") != "1" {
		t.Fatal("use verify-r21-contracts script for real binary/PostgreSQL/LDAPS contract replay")
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
	dbURL := os.Getenv("TEST_DATABASE_URL")
	parsed, err := url.Parse(dbURL)
	if err != nil || parsed.Path != "/verification" {
		t.Fatal("scratch /verification database required")
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatal("scratch database unavailable")
	}
	defer db.Close()
	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal("signing fixture failed")
	}
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &signingKey.PublicKey, KeyID: "scratch", Algorithm: "RS256", Use: "sig"}}})
	}))
	defer issuer.Close()

	auth := map[string]string{"bot-member": "Basic " + base64.StdEncoding.EncodeToString([]byte("bot:"+botPassword))}
	for actor, subject := range map[string]string{"human-member": "fixture-member", "human-peer": "fixture-peer"} {
		signed := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": issuer.URL, "aud": "scratch-api", "sub": subject, "preferred_username": subject, "exp": time.Now().Add(time.Hour).Unix()})
		signed.Header["kid"] = "scratch"
		value, e := signed.SignedString(signingKey)
		if e != nil {
			t.Fatal("token fixture failed")
		}
		auth[actor] = "Bearer " + value
	}
	botExternalID := graphql(`{user(userId:"bot"){uuid}}`, nil)["user"].(map[string]any)["uuid"].(string)
	botID := "66666666-6666-4666-8666-666666666666"
	apiPort := port()
	apiURL := fmt.Sprintf("http://127.0.0.1:%d", apiPort)
	target, _ := url.Parse(apiURL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) { director(r); r.Header.Set("X-Forwarded-Proto", "https") }
	ingress := httptest.NewServer(proxy)
	defer ingress.Close()
	readerFile := write("reader-secret", []byte(readerPassword))
	var process *exec.Cmd
	stop := func() {
		if process != nil {
			_ = process.Process.Kill()
			_ = process.Wait()
			process = nil
		}
	}
	defer stop()
	start := func() error {
		process = exec.Command("/meowgram-server")
		process.Env = append(os.Environ(), "ENVIRONMENT=development", "APP_DOMAIN=localhost", "LOG_LEVEL=error", fmt.Sprintf("PORT=%d", apiPort), "DATABASE_URL="+dbURL, "MIGRATIONS_PATH=/migrations",
			"AUTHELIA_ISSUER="+issuer.URL, "AUTHELIA_JWKS_URL="+issuer.URL+"/jwks", "AUTHELIA_AUDIENCE=scratch-api", "CORS_ORIGINS="+ingress.URL,
			"BOT_BASIC_AUTH_ENABLED=true", fmt.Sprintf("BOT_LDAP_URL=ldaps://localhost:%d", tlsPort), "BOT_LDAP_BASE_DN=ou=people,"+base, "BOT_LDAP_READER_DN=uid=reader,ou=people,"+base,
			"BOT_LDAP_READER_PASSWORD_FILE="+readerFile, "BOT_LDAP_ALLOWED_GROUP=cn=bots,ou=groups,"+base, "BOT_LDAP_DIRECTORY_NAMESPACE=scratch-r21-contract", "BOT_LDAP_CA_FILE="+ca, "BOT_LDAP_TIMEOUT=3s", "BOT_TRUSTED_PROXY_CIDRS=127.0.0.1/32")
		if err := process.Start(); err != nil {
			return fmt.Errorf("server process failed to start")
		}
		for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
			r, e := client.Get(apiURL + "/healthz")
			if e == nil {
				r.Body.Close()
				if r.StatusCode == 200 {
					return nil
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
		return fmt.Errorf("server readiness timed out (logs suppressed)")
	}
	if err := start(); err != nil {
		t.Fatal(err)
	}
	stop() // Real binary applies its shipped migrations.
	seed := func() error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		statements := []string{
			`TRUNCATE users,bot_principals,conversations RESTART IDENTITY CASCADE`,
			`INSERT INTO users(authelia_sub,username) VALUES ('fixture-member','fixture-member'),('fixture-peer','fixture-peer'),('fixture-outsider','fixture-outsider')`,
			`INSERT INTO bot_principals(id,username) VALUES ('66666666-6666-4666-8666-666666666666','bot')`,
			`INSERT INTO conversations(id,name,created_by_kind,created_by_id) VALUES ('11111111-1111-4111-8111-111111111111','Group A','human','fixture-member'),('22222222-2222-4222-8222-222222222222','Group B','human','fixture-outsider'),('44444444-4444-4444-8444-444444444444','No humans','human','fixture-member'),('55555555-5555-4555-8555-555555555555','Group C','human','fixture-member')`,
			`INSERT INTO conversation_members(conversation_id,member_kind,human_sub) VALUES ('11111111-1111-4111-8111-111111111111','human','fixture-member'),('11111111-1111-4111-8111-111111111111','human','fixture-peer'),('22222222-2222-4222-8222-222222222222','human','fixture-outsider'),('55555555-5555-4555-8555-555555555555','human','fixture-member')`,
			`INSERT INTO conversation_members(conversation_id,member_kind,bot_id) SELECT id,'bot','66666666-6666-4666-8666-666666666666'::uuid FROM conversations WHERE id<>'22222222-2222-4222-8222-222222222222'`,
			`INSERT INTO messages(id,sender_id,text_content,created_at,channel_id) VALUES ('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','fixture-member','lounge positive control','2026-01-01T00:00:00Z',NULL),('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb','fixture-member','A positive control','2026-01-01T00:00:01Z','11111111-1111-4111-8111-111111111111'),('cccccccc-cccc-4ccc-8ccc-cccccccccccc','fixture-outsider','B private canary','2026-01-01T00:00:02Z','22222222-2222-4222-8222-222222222222'),('dddddddd-dddd-4ddd-8ddd-dddddddddddd','fixture-member','no-human private canary','2026-01-01T00:00:03Z','44444444-4444-4444-8444-444444444444')`,
		}
		for i, statement := range statements {
			if _, err := tx.Exec(statement); err != nil {
				return fmt.Errorf("seed statement %d failed: %w", i, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO bot_directory_identities(provider,directory_namespace,external_id,principal_id) VALUES ('lldap','scratch-r21-contract',$1,$2)`, botExternalID, botID); err != nil {
			return fmt.Errorf("seed bot directory mapping failed")
		}
		return tx.Commit()
	}
	var resetMu sync.Mutex
	resets := 0
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resetMu.Lock()
		defer resetMu.Unlock()
		var request struct {
			Seed string `json:"seed"`
			Case string `json:"case"`
		}
		if r.Method != "POST" || r.URL.Path != "/reset" || json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&request) != nil || request.Seed != "lounge-and-groups-v1" || request.Case == "" {
			http.Error(w, "invalid disposable reset", 400)
			return
		}
		stop()
		if err := seed(); err != nil {
			t.Logf("seed reset failed: %v", err)
			http.Error(w, "seed reset failed", 500)
			return
		}
		if err := start(); err != nil {
			t.Logf("server reset failed: %v", err)
			http.Error(w, "server reset failed", 500)
			return
		}
		resets++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(auth)
	}))
	defer control.Close()
	runner := exec.Command("/r21-contracts.test", "-test.v", "-test.run=^TestHTTPContract$", "-test.timeout=240s")
	runner.Dir = "/contracts"
	runner.Env = append(os.Environ(), "MEOWGRAM_DISPOSABLE_R21_CONTRACT=1", "R21_CONTRACT_URL="+ingress.URL, "R21_CONTRACT_CONTROL_URL="+control.URL)
	runner.Stdout = os.Stdout
	runner.Stderr = os.Stderr
	if err := runner.Run(); err != nil {
		t.Fatal("frozen HTTP contract replay failed")
	}
	resetMu.Lock()
	defer resetMu.Unlock()
	if resets != 78 {
		t.Fatalf("contract reset count=%d want=78", resets)
	}
	t.Logf("real-binary R2.1 HTTP contract: %d fixtures passed; fresh process and database seed per case", resets)
}

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
	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/v5/stdlib"
	"meowgram/server/internal/model"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

// Runs only in the disposable pinned-provider image; no live configuration is read.
func TestAPIEndToEnd(t *testing.T) {
	if os.Getenv("MEOWGRAM_DISPOSABLE_API") != "1" {
		t.Skip("use verify-api-acceptance script for real server/PostgreSQL/LDAPS integration")
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
		if !ok || result["database/sql"] != nil {
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
	signed := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": issuer.URL, "aud": "scratch-api", "sub": "scratch-human", "preferred_username": "human", "exp": time.Now().Add(time.Hour).Unix()})
	signed.Header["kid"] = "scratch"
	bearer, err := signed.SignedString(signingKey)
	if err != nil {
		t.Fatal("token fixture failed")
	}
	apiPort := port()
	apiURL := fmt.Sprintf("http://127.0.0.1:%d", apiPort)
	target, _ := url.Parse(apiURL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) { director(r); r.Header.Set("X-Forwarded-Proto", "https") }
	ingress := httptest.NewTLSServer(proxy)
	defer ingress.Close()
	process := exec.Command("/meowgram-server")
	process.Env = append(os.Environ(), "ENVIRONMENT=development", "APP_DOMAIN=localhost", "LOG_LEVEL=error", fmt.Sprintf("PORT=%d", apiPort), "DATABASE_URL="+dbURL, "MIGRATIONS_PATH=/migrations",
		"AUTHELIA_ISSUER="+issuer.URL, "AUTHELIA_JWKS_URL="+issuer.URL+"/jwks", "AUTHELIA_AUDIENCE=scratch-api", "CORS_ORIGINS="+ingress.URL,
		"BOT_BASIC_AUTH_ENABLED=true", fmt.Sprintf("BOT_LDAP_URL=ldaps://localhost:%d", tlsPort), "BOT_LDAP_BASE_DN=ou=people,"+base, "BOT_LDAP_READER_DN=uid=reader,ou=people,"+base,
		"BOT_LDAP_READER_PASSWORD_FILE="+write("reader-secret", []byte(readerPassword)), "BOT_LDAP_ALLOWED_GROUP=cn=bots,ou=groups,"+base, "BOT_LDAP_DIRECTORY_NAMESPACE=scratch-acceptance", "BOT_LDAP_CA_FILE="+ca, "BOT_LDAP_TIMEOUT=3s", "BOT_TRUSTED_PROXY_CIDRS=127.0.0.1/32",
		"API_RATE_PER_MINUTE=30", "API_RATE_BURST=30", "GLOBAL_RATE_PER_MINUTE=6000", "GLOBAL_RATE_BURST=200", "MESSAGE_RATE_PER_MINUTE=60", "MESSAGE_RATE_BURST=30")
	if process.Start() != nil {
		t.Fatal("server process failed to start")
	}
	t.Cleanup(func() { _ = process.Process.Kill(); _ = process.Wait() })
	ready := false
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		r, e := client.Get(apiURL + "/healthz")
		if e == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("server binary not ready (logs suppressed)")
	}
	httpClient := ingress.Client()
	httpClient.Timeout = 10 * time.Second
	call := func(method, path, user, password, key, body string) (int, []byte, http.Header) {
		t.Helper()
		r, e := http.NewRequest(method, ingress.URL+path, strings.NewReader(body))
		if e != nil {
			t.Fatal("request construction")
		}
		if user == "human" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		} else {
			r.SetBasicAuth(user, password)
		}
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		response, e := httpClient.Do(r)
		if e != nil {
			t.Fatal("API request failed (details suppressed)")
		}
		defer response.Body.Close()
		data, e := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if e != nil {
			t.Fatal("API response read failed")
		}
		return response.StatusCode, data, response.Header
	}
	expect := func(want int, method, path, user, password, key, body string) []byte {
		t.Helper()
		status, data, _ := call(method, path, user, password, key, body)
		if status != want {
			t.Fatalf("%s %s status %d want %d", method, path, status, want)
		}
		return data
	}
	syncPath := "/api/messages/sync?after=1970-01-01T00:00:00Z"
	t.Run("wrong_password_401", func(t *testing.T) { expect(401, "GET", syncPath, "bot", secret(), "", "") })
	t.Run("nonmember_403", func(t *testing.T) { expect(403, "GET", syncPath, "reader", readerPassword, "", "") })
	t.Run("human_oidc_send_sync", func(t *testing.T) {
		data := expect(201, "POST", "/api/messages", "human", "", "human-key", `{"text_content":"human HTTP"}`)
		var m model.Message
		if json.Unmarshal(data, &m) != nil || m.SenderKind != "human" || m.SenderID != "scratch-human" {
			t.Fatal("human identity changed")
		}
		data = expect(200, "GET", syncPath, "human", "", "", "")
		if !bytes.Contains(data, []byte(m.ID)) {
			t.Fatal("human sync missing send")
		}
	})
	ticketData := expect(200, "POST", "/api/ws-ticket", "human", "", "", "")
	var ticket struct {
		Ticket string `json:"ticket"`
	}
	if json.Unmarshal(ticketData, &ticket) != nil || ticket.Ticket == "" {
		t.Fatal("ticket absent")
	}
	dialer := websocket.Dialer{TLSClientConfig: httpClient.Transport.(*http.Transport).TLSClientConfig, HandshakeTimeout: 5 * time.Second}
	ws, _, err := dialer.Dial("wss"+strings.TrimPrefix(ingress.URL, "https")+"/ws?ticket="+url.QueryEscape(ticket.Ticket), http.Header{"Origin": []string{ingress.URL}})
	if err != nil {
		t.Fatal("human WebSocket connection failed")
	}
	defer ws.Close()
	readMessage := func(text, kind string) {
		t.Helper()
		ws.SetReadDeadline(time.Now().Add(5 * time.Second))
		for i := 0; i < 20; i++ {
			var m model.WSMessage
			if ws.ReadJSON(&m) != nil {
				t.Fatal("WebSocket message missing")
			}
			if m.TextContent == text {
				if m.SenderKind != kind {
					t.Fatal("WebSocket sender kind changed")
				}
				return
			}
		}
		t.Fatal("WebSocket message not found")
	}
	t.Run("human_websocket_send", func(t *testing.T) {
		if ws.WriteMessage(websocket.TextMessage, []byte("human WS")) != nil {
			t.Fatal("WS write failed")
		}
		readMessage("human WS", "human")
	})
	var sent []byte
	var botMessage model.Message
	t.Run("bot_send_201_live_kind", func(t *testing.T) {
		sent = expect(201, "POST", "/api/messages", "bot", botPassword, "pourover-reply-original", `{"text_content":"bot HTTP"}`)
		if json.Unmarshal(sent, &botMessage) != nil || botMessage.SenderKind != "bot" || botMessage.Username != "bot" || botMessage.SenderID == "" {
			t.Fatal("bot identity missing")
		}
		readMessage("bot HTTP", "bot")
	})
	t.Run("identical_replay_one_row", func(t *testing.T) {
		replay := expect(201, "POST", "/api/messages", "bot", botPassword, "pourover-reply-original", `{"text_content":"bot HTTP"}`)
		if !bytes.Equal(sent, replay) {
			t.Fatal("replay changed response")
		}
		var count int
		if db.QueryRow(`SELECT count(*) FROM messages WHERE text_content='bot HTTP'`).Scan(&count) != nil || count != 1 {
			t.Fatal("replay created duplicate")
		}
	})
	t.Run("changed_body_409", func(t *testing.T) {
		expect(409, "POST", "/api/messages", "bot", botPassword, "pourover-reply-original", `{"text_content":"changed"}`)
	})
	t.Run("bot_sync_kind", func(t *testing.T) {
		data := expect(200, "GET", syncPath, "bot", botPassword, "", "")
		var messages []model.Message
		if json.Unmarshal(data, &messages) != nil {
			t.Fatal("invalid sync")
		}
		for _, m := range messages {
			if m.ID == botMessage.ID {
				if m.SenderKind != "bot" || m.SenderID != botMessage.SenderID {
					t.Fatal("sync kind/identity wrong")
				}
				return
			}
		}
		t.Fatal("bot absent from sync")
	})
	t.Run("channel_id_400", func(t *testing.T) {
		expect(400, "POST", "/api/messages", "bot", botPassword, "channel", `{"text_content":"x","channel_id":"lounge"}`)
	})
	t.Run("basic_route_isolation", func(t *testing.T) {
		expect(401, "POST", "/api/ws-ticket", "bot", botPassword, "", "")
		expect(401, "HEAD", "/api/messages/sync", "bot", botPassword, "", "")
	})
	t.Run("group_removal_next_request_403", func(t *testing.T) {
		membership("bot", group, false)
		expect(403, "GET", syncPath, "bot", botPassword, "", "")
		membership("bot", group, true)
	})
	t.Run("burst_429_integer_retry", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			status, _, headers := call("GET", syncPath, "bot", botPassword, "", "")
			if status == 429 {
				seconds, e := strconv.Atoi(headers.Get("Retry-After"))
				if e != nil || seconds < 1 {
					t.Fatal("invalid Retry-After")
				}
				return
			}
			if status != 200 {
				t.Fatalf("burst status %d", status)
			}
		}
		t.Fatal("burst never throttled")
	})
	t.Run("directory_down_503_retry", func(t *testing.T) {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		status, _, headers := call("GET", syncPath, "bot", botPassword, "", "")
		seconds, e := strconv.Atoi(headers.Get("Retry-After"))
		if status != 503 || e != nil || seconds < 1 {
			t.Fatalf("directory outage status %d or invalid retry", status)
		}
	})
	t.Run("human_unaffected_by_directory_down", func(t *testing.T) {
		expect(200, "GET", syncPath, "human", "", "", "")
		expect(201, "POST", "/api/messages", "human", "", "human-after-outage", `{"text_content":"human after outage"}`)
	})
	t.Run("startup_migrations_clean", func(t *testing.T) {
		// Inspect the same directory shipped to the real binary in this image.
		files, err := filepath.Glob("/migrations/*.up.sql")
		if err != nil || len(files) == 0 {
			t.Fatal("missing shipped migrations")
		}
		var expected int
		for _, file := range files {
			prefix, _, ok := strings.Cut(filepath.Base(file), "_")
			v, err := strconv.Atoi(prefix)
			if !ok || err != nil {
				t.Fatal("invalid shipped migration name")
			}
			if v > expected {
				expected = v
			}
		}
		var version int
		var dirty bool
		if db.QueryRow(`SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty) != nil || version != expected || dirty {
			t.Fatal("startup migration state wrong")
		}
	})
	managementAcceptance(t, issuer.URL, signingKey, httpClient, ingress.URL, db)
}

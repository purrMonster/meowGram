package integration

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
)

// The dedicated image contains both actual server binaries, their migration files,
// and PostgreSQL tools. Never point this destructive drill at an existing DB.
func TestReleaseRollback(t *testing.T) {
	if os.Getenv("MEOWGRAM_DISPOSABLE_ROLLBACK") != "1" {
		t.Skip("use verify-rollback script with disposable PostgreSQL and v1.1.0/current binaries")
	}
	dbURL := os.Getenv("TEST_DATABASE_URL")
	parsed, err := url.Parse(dbURL)
	if err != nil || parsed.Hostname() != "db" || parsed.Path != "/verification" {
		t.Fatal("dedicated scratch db/verification required")
	}
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "rollback", Algorithm: "RS256", Use: "sig"}}})
	}))
	defer issuer.Close()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": issuer.URL, "aud": "rollback-api", "sub": "rollback-human", "preferred_username": "human", "exp": time.Now().Add(time.Hour).Unix()})
	token.Header["kid"] = "rollback"
	bearer, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	start := func(binary, migrations, database string) (string, func()) {
		t.Helper()
		listener, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		address := listener.Addr().String()
		listener.Close()
		_, port, _ := net.SplitHostPort(address)
		cmd := exec.Command(binary)
		cmd.Env = append(os.Environ(), "DATABASE_URL="+database, "MIGRATIONS_PATH="+migrations, "PORT="+port, "ENVIRONMENT=development", "APP_DOMAIN=localhost", "LOG_LEVEL=error", "AUTHELIA_ISSUER="+issuer.URL, "AUTHELIA_JWKS_URL="+issuer.URL, "AUTHELIA_AUDIENCE=rollback-api", "BOT_BASIC_AUTH_ENABLED=false", "CORS_ORIGINS=http://"+address)
		if e = cmd.Start(); e != nil {
			t.Fatal(e)
		}
		stopped := false
		stop := func() {
			if !stopped {
				stopped = true
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		}
		t.Cleanup(stop)
		for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
			response, e := client.Get("http://" + address + "/healthz")
			if e == nil {
				response.Body.Close()
				if response.StatusCode == 200 {
					return "http://" + address, stop
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		stop()
		t.Fatal("server did not become healthy")
		return "", stop
	}
	call := func(endpoint, method, path, body string) (int, []byte) {
		t.Helper()
		request, _ := http.NewRequest(method, endpoint+path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+bearer)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "rollback-human-http")
		response, e := client.Do(request)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		data, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		return response.StatusCode, data
	}
	sendWS := func(endpoint, text string) {
		t.Helper()
		status, data := call(endpoint, "POST", "/api/ws-ticket", "")
		if status != 200 {
			t.Fatalf("ticket status %d", status)
		}
		var ticket struct {
			Ticket string `json:"ticket"`
		}
		if json.Unmarshal(data, &ticket) != nil || ticket.Ticket == "" {
			t.Fatal("missing ticket")
		}
		ws, _, e := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(endpoint, "http")+"/ws?ticket="+url.QueryEscape(ticket.Ticket), http.Header{"Origin": []string{endpoint}})
		if e != nil {
			t.Fatal(e)
		}
		defer ws.Close()
		if e = ws.WriteMessage(websocket.TextMessage, []byte(text)); e != nil {
			t.Fatal(e)
		}
		ws.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			var message struct {
				Text string `json:"text_content"`
			}
			if e = ws.ReadJSON(&message); e != nil {
				t.Fatal(e)
			}
			if message.Text == text {
				break
			}
		}
	}
	history := func(endpoint string, want int, contains string) {
		t.Helper()
		status, data := call(endpoint, "GET", "/api/messages/sync?after=1970-01-01T00:00:00Z", "")
		if status != want || !bytes.Contains(data, []byte(contains)) {
			t.Fatalf("history status=%d want=%d; expected marker present=%v", status, want, bytes.Contains(data, []byte(contains)))
		}
	}
	query := func(statement string) {
		t.Helper()
		if _, e := db.Exec(statement); e != nil {
			t.Fatal(e)
		}
	}
	count := func(table string, want int) {
		t.Helper()
		var n int
		if e := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != want {
			t.Fatalf("%s count=%d want=%d error=%v", table, n, want, e)
		}
	}
	version := func(want int) {
		t.Helper()
		var n int
		var dirty bool
		if e := db.QueryRow("SELECT version,dirty FROM schema_migrations").Scan(&n, &dirty); e != nil || n != want || dirty {
			t.Fatalf("version=%d dirty=%v error=%v", n, dirty, e)
		}
	}
	command := func(name string, args ...string) {
		t.Helper()
		c := exec.Command(name, args...)
		if e := c.Run(); e != nil {
			t.Fatalf("scratch %s failed: %v", name, e)
		}
	}
	endpoint, stop := start("/previous-server", "/previous-migrations", dbURL)
	sendWS(endpoint, "before upgrade")
	history(endpoint, 200, "before upgrade")
	stop()
	version(4)
	dump := filepath.Join(t.TempDir(), "pre-upgrade.dump")
	command("pg_dump", "--dbname="+dbURL, "--format=custom", "--file="+dump)
	endpoint, stop = start("/current-server", "/current-migrations", dbURL)
	status, _ := call(endpoint, "POST", "/api/messages", `{"text_content":"after upgrade"}`)
	if status != 201 {
		t.Fatalf("current send status %d", status)
	}
	stop()
	version(6)
	count("messages", 2)
	count("message_idempotency", 1)
	t.Run("previous_binary_rejects_version_six", func(t *testing.T) {
		c := exec.Command("/previous-server")
		c.Env = append(os.Environ(), "DATABASE_URL="+dbURL, "MIGRATIONS_PATH=/previous-migrations", "ENVIRONMENT=development", "AUTHELIA_ISSUER="+issuer.URL, "AUTHELIA_JWKS_URL="+issuer.URL, "AUTHELIA_AUDIENCE=rollback-api")
		output, e := c.CombinedOutput()
		if e == nil || !bytes.Contains(output, []byte("no migration found for version 6")) {
			t.Fatal("expected version-ahead migration startup failure")
		}
	})
	t.Run("keep_data_force_four_previous_history_and_send", func(t *testing.T) {
		query("UPDATE schema_migrations SET version=4, dirty=false")
		endpoint, stop = start("/previous-server", "/previous-migrations", dbURL)
		history(endpoint, 200, "after upgrade")
		sendWS(endpoint, "during rollback")
		history(endpoint, 200, "during rollback")
		stop()
		version(4)
		count("messages", 3)
		count("message_idempotency", 1)
	})
	t.Run("roll_forward_reapplies_five_six_and_preserves_rows", func(t *testing.T) {
		endpoint, stop = start("/current-server", "/current-migrations", dbURL)
		history(endpoint, 200, "during rollback")
		stop()
		version(6)
		count("messages", 3)
		count("message_idempotency", 1)
	})
	t.Run("bot_row_breaks_previous_history_after_force_four", func(t *testing.T) {
		query("WITH bot AS (INSERT INTO bot_principals(username) VALUES ('rollback-bot') RETURNING id) INSERT INTO messages(bot_principal_id,text_content) SELECT id,'bot marker' FROM bot")
		query("UPDATE schema_migrations SET version=4, dirty=false")
		endpoint, stop = start("/previous-server", "/previous-migrations", dbURL)
		history(endpoint, 500, "")
		stop()
		endpoint, stop = start("/current-server", "/current-migrations", dbURL)
		history(endpoint, 200, "bot marker")
		stop()
		version(6)
	})
	t.Run("full_restore_previous_backup_loses_post_upgrade_messages", func(t *testing.T) {
		command("createdb", "--maintenance-db="+dbURL, "rollback_restore")
		restoreURL := *parsed
		restoreURL.Path = "/rollback_restore"
		command("pg_restore", "--dbname="+restoreURL.String(), "--no-owner", "--exit-on-error", dump)
		endpoint, stop = start("/previous-server", "/previous-migrations", restoreURL.String())
		history(endpoint, 200, "before upgrade")
		status, data := call(endpoint, "GET", "/api/messages/sync?after=1970-01-01T00:00:00Z", "")
		if status != 200 || bytes.Contains(data, []byte("after upgrade")) || bytes.Contains(data, []byte("bot marker")) {
			t.Fatal("restore retained post-backup messages")
		}
		sendWS(endpoint, "after restore")
		history(endpoint, 200, "after restore")
		stop()
	})
}

package handler

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/config"
	"meowgram/server/internal/middleware"
	"meowgram/server/internal/model"
	"meowgram/server/internal/ratelimit"
	"meowgram/server/internal/repository"
	"meowgram/server/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type sendDirectoryFunc func(context.Context, string, string) (botauth.Identity, error)

func (f sendDirectoryFunc) Authenticate(c context.Context, u, p string) (botauth.Identity, error) {
	return f(c, u, p)
}

type sendPublishFunc func(*model.WSMessage) bool

func (f sendPublishFunc) Publish(m *model.WSMessage) bool { return f(m) }

type sendBudgetFunc func(string) (bool, time.Duration)

func (f sendBudgetFunc) Allow(k string) (bool, time.Duration) { return f(k) }

func TestHTTPSendContract(t *testing.T) {
	db, m, _ := testutil.Database(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "send", Algorithm: "RS256", Use: "sig"}}})
	}))
	defer issuer.Close()
	cfg := &config.Config{AutheliaIssuer: issuer.URL, AutheliaJWKSURL: issuer.URL + "/jwks", AutheliaAudience: "send-test", CORSOrigins: []string{"https://example.home.arpa"}}
	verifier, err := auth.NewOIDCVerifier(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": issuer.URL, "aud": "send-test", "sub": "human-send", "preferred_username": "human", "exp": time.Now().Add(time.Hour).Unix()})
	token.Header["kid"] = "send"
	bearer, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	human := auth.Middleware(verifier, repository.NewUserRepository(db), logger)
	var directoryStatus, authCalls, writes, published atomic.Int32
	var writeDenied, apiDenied, ingressDenied, stopped atomic.Bool
	identity := botauth.Identity{Namespace: "synthetic", ExternalID: uuid.NewString(), Username: "bot"}
	gate := &botauth.Gate{Directory: sendDirectoryFunc(func(_ context.Context, u, p string) (botauth.Identity, error) {
		authCalls.Add(1)
		switch directoryStatus.Load() {
		case 403:
			return botauth.Identity{}, botauth.ErrForbidden
		case 503:
			return botauth.Identity{}, botauth.ErrUnavailable
		}
		if u != "bot" || p != "synthetic" {
			return botauth.Identity{}, botauth.ErrCredentials
		}
		return identity, nil
	}), Resolve: botauth.RepositoryResolver(repository.NewBotRepository(db)), Logger: logger, Timeout: 5 * time.Second,
		Ingress: sendBudgetFunc(func(string) (bool, time.Duration) { return !ingressDenied.Load(), time.Millisecond }),
		API:     sendBudgetFunc(func(string) (bool, time.Duration) { return !apiDenied.Load(), time.Millisecond })}
	budget := sendBudgetFunc(func(string) (bool, time.Duration) { writes.Add(1); return !writeDenied.Load(), time.Millisecond })
	publisher := sendPublishFunc(func(m *model.WSMessage) bool {
		if m.SenderKind != "bot" && m.SenderKind != "human" {
			t.Error("missing publication kind")
		}
		published.Add(1)
		return !stopped.Load()
	})
	newRoute := func(enabled bool) http.Handler {
		mux := http.NewServeMux()
		var g *botauth.Gate
		if enabled {
			g = gate
		}
		if err := RegisterSendRoute(mux, "/api/messages", chat.NewMessageService(repository.NewMessageRepository(db), publisher, budget), 168*time.Hour, human, g); err != nil {
			t.Fatal(err)
		}
		return middleware.CORS(cfg)(mux)
	}
	route := newRoute(true)
	request := func(h http.Handler, key, body, credential string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "https://example.home.arpa/api/messages", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if credential == "human" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		} else if credential != "none" {
			r.SetBasicAuth("bot", credential)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	const body = `{"text_content":"@human hello 🐾"}`
	op := "pourover-reply-" + uuid.NewString()
	var saved string
	t.Run("concurrent identical response one publication", func(t *testing.T) {
		const n = 20
		results := make(chan *httptest.ResponseRecorder, n)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); results <- request(route, op, body, "synthetic") }()
		}
		wg.Wait()
		close(results)
		for w := range results {
			if w.Code != 201 {
				t.Fatalf("status %d %s", w.Code, w.Body)
			}
			if saved == "" {
				saved = w.Body.String()
			}
			if w.Body.String() != saved {
				t.Fatal("non-identical replay")
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatal("response headers changed")
			}
		}
		if published.Load() != 1 || writes.Load() != 1 || authCalls.Load() != n {
			t.Fatalf("published=%d writes=%d auth=%d", published.Load(), writes.Load(), authCalls.Load())
		}
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM messages`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("rows=%d %v", count, err)
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(saved), &msg); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"id", "sender_id", "sender_kind", "username", "text_content", "created_at"} {
			if _, ok := msg[field]; !ok {
				t.Fatal("missing " + field)
			}
		}
		if msg["sender_kind"] != "bot" || msg["username"] != "bot" {
			t.Fatal(msg)
		}
	})
	t.Run("restart replay conflict and normalization", func(t *testing.T) {
		restarted := newRoute(true)
		w := request(restarted, op, "{\n \"text_content\" : \"@human hello 🐾\" }", "synthetic")
		if w.Code != 201 || w.Body.String() != saved {
			t.Fatal("restart replay changed")
		}
		for _, text := range []string{"different", "", " @human hello 🐾"} {
			raw, _ := json.Marshal(map[string]string{"text_content": text})
			if w := request(route, op, string(raw), "synthetic"); w.Code != 409 {
				t.Fatalf("changed text status %d", w.Code)
			}
		}
	})
	t.Run("reauthentication and isolation", func(t *testing.T) {
		directoryStatus.Store(403)
		w := request(route, op, body, "synthetic")
		if w.Code != 403 {
			t.Fatal("revoked replay accepted")
		}
		directoryStatus.Store(0)
		if w := request(route, op, body, "wrong"); w.Code != 401 {
			t.Fatal("bad password accepted")
		}
		if w := request(route, op, body, "none"); w.Code != 401 {
			t.Fatal("unauthenticated accepted")
		}
		if w := request(newRoute(false), op, body, "synthetic"); w.Code != 401 {
			t.Fatal("disabled Basic accepted")
		}
		w = request(route, op, body, "human")
		if w.Code != 201 || w.Body.String() == saved {
			t.Fatalf("human/key isolation %d", w.Code)
		}
		var msg model.Message
		json.Unmarshal(w.Body.Bytes(), &msg)
		if msg.SenderKind != "human" || msg.SenderID != "human-send" {
			t.Fatal("human sender changed")
		}
		for _, path := range []string{"/api/ws-ticket", "/api/push/devices", "/api/media", "/api/messages/sync"} {
			r := httptest.NewRequest("POST", "https://example.home.arpa"+path, strings.NewReader(body))
			r.SetBasicAuth("bot", "synthetic")
			w := httptest.NewRecorder()
			route.ServeHTTP(w, r)
			if w.Code >= 200 && w.Code < 300 {
				t.Fatal("Basic escaped route")
			}
		}
	})
	t.Run("backoff", func(t *testing.T) {
		for _, which := range []string{"directory", "api", "ingress", "write"} {
			directoryStatus.Store(0)
			apiDenied.Store(false)
			ingressDenied.Store(false)
			writeDenied.Store(false)
			expected := 429
			switch which {
			case "directory":
				directoryStatus.Store(503)
				expected = 503
			case "api":
				apiDenied.Store(true)
			case "ingress":
				ingressDenied.Store(true)
			case "write":
				writeDenied.Store(true)
			}
			w := request(route, "limited-"+which, body, "synthetic")
			seconds, err := strconv.Atoi(w.Header().Get("Retry-After"))
			if w.Code != expected || err != nil || seconds < 1 {
				t.Fatalf("%s: status %d retry %q", which, w.Code, w.Header().Get("Retry-After"))
			}
		}
		directoryStatus.Store(0)
		apiDenied.Store(false)
		ingressDenied.Store(false)
		writeDenied.Store(false)
	})
	t.Run("validation", func(t *testing.T) {
		for _, bad := range []string{`{"text_content":"x","channel_id":"lounge"}`, `{}`, `null`, `{"text_content":3}`, `{"text_content":"x","sender_id":"spoof"}`, `{"text_content":"x","id":"spoof"}`, `{"text_content":"x"} {}`, `{"text_content":" "}`, `{"text_content":"` + strings.Repeat("🐾", 4001) + `"}`} {
			if w := request(route, uuid.NewString(), bad, "synthetic"); w.Code != 400 {
				t.Fatalf("bad body status %d", w.Code)
			}
		}
		for _, bad := range []string{"", strings.Repeat("x", 256), "bad\tkey"} {
			if w := request(route, bad, body, "synthetic"); w.Code != 400 {
				t.Fatalf("bad key status %d", w.Code)
			}
		}
		if w := request(route, "huge", `{"text_content":"`+strings.Repeat("x", 40000)+`"}`, "synthetic"); w.Code != 413 {
			t.Fatalf("large status %d", w.Code)
		}
	})
	t.Run("failure rollback and durable publication failure", func(t *testing.T) {
		before := published.Load()
		var rows int
		db.QueryRow(`SELECT count(*) FROM messages`).Scan(&rows)
		if _, err := db.Exec(`ALTER TABLE message_idempotency ADD CONSTRAINT fail_test CHECK (operation_key <> 'fail')`); err != nil {
			t.Fatal(err)
		}
		w := request(route, "fail", body, "synthetic")
		if w.Code != 503 || w.Header().Get("Retry-After") == "" {
			t.Fatalf("db error %d", w.Code)
		}
		var after int
		db.QueryRow(`SELECT count(*) FROM messages`).Scan(&after)
		if after != rows || published.Load() != before {
			t.Fatal("failed write escaped transaction")
		}
		stopped.Store(true)
		w = request(route, "stopped", body, "synthetic")
		if w.Code != 201 {
			t.Fatal("durable send lost")
		}
		replay := request(route, "stopped", body, "synthetic")
		if replay.Body.String() != w.Body.String() || published.Load() != before+1 {
			t.Fatal("durable retry republished")
		}
		stopped.Store(false)
	})
	t.Run("HTTP and WebSocket share human write budget", func(t *testing.T) {
		service := chat.NewMessageService(repository.NewMessageRepository(db), publisher, ratelimit.New(1, 1))
		if _, err := service.Send(context.Background(), "human-send", "human", "websocket first"); err != nil {
			t.Fatal(err)
		}
		mux := http.NewServeMux()
		if err := RegisterSendRoute(mux, "/api/messages", service, 168*time.Hour, human, gate); err != nil {
			t.Fatal(err)
		}
		w := request(mux, "shared-budget", body, "human")
		if w.Code != 429 {
			t.Fatalf("transport reset write budget: %d", w.Code)
		}
	})
	t.Run("user deletion cascades idempotency", func(t *testing.T) {
		w := request(route, "delete-user", body, "human")
		if w.Code != 201 {
			t.Fatalf("send: %d %s", w.Code, w.Body)
		}
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM message_idempotency WHERE sender_kind='human' AND sender_id='human-send'`).Scan(&count); err != nil || count == 0 {
			t.Fatalf("missing sent key: %d %v", count, err)
		}
		if _, err := db.Exec(`DELETE FROM users WHERE authelia_sub='human-send'`); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM message_idempotency WHERE sender_kind='human' AND sender_id='human-send'`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("orphan keys: %d %v", count, err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM messages WHERE sender_id='human-send'`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("orphan messages: %d %v", count, err)
		}
	})
	t.Run("cors", func(t *testing.T) {
		r := httptest.NewRequest("OPTIONS", "https://example.home.arpa/api/messages", nil)
		r.Header.Set("Origin", "https://example.home.arpa")
		r.Header.Set("Access-Control-Request-Headers", "Idempotency-Key")
		w := httptest.NewRecorder()
		route.ServeHTTP(w, r)
		if w.Code != 204 || !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Idempotency-Key") {
			t.Fatal("CORS missing key header")
		}
	})

}

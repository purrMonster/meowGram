package main

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/handler"
	"meowgram/server/internal/model"
	"meowgram/server/internal/ratelimit"
	"meowgram/server/internal/repository"
	"meowgram/server/internal/testutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type directoryFunc func(context.Context, string, string) (botauth.Identity, error)

func (f directoryFunc) Authenticate(c context.Context, u, p string) (botauth.Identity, error) {
	return f(c, u, p)
}

type publishFunc func(*model.WSMessage) bool

func (f publishFunc) Publish(m *model.WSMessage) bool { return f(m) }

func TestExampleScratchRestartAndBackoff(t *testing.T) {
	db, m, _ := testutil.Database(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewMessageRepository(db)
	bots := repository.NewBotRepository(db)
	identity := botauth.Identity{Namespace: "example", ExternalID: uuid.NewString(), Username: "example-bot"}
	principal, err := bots.Resolve(context.Background(), identity.Namespace, identity.ExternalID, identity.Username)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(authelia_sub,username) VALUES($1,'example-bot')`, principal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO messages(sender_id,text_content,created_at) VALUES($1,'human sharing bot name and ID','2020-01-01T00:00:00Z')`, principal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO messages(bot_principal_id,text_content,created_at) VALUES($1,'own message','2020-01-02T00:00:00Z')`, principal.ID); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var authCalls, published, postCalls atomic.Int32
	gate := &botauth.Gate{Directory: directoryFunc(func(_ context.Context, u, p string) (botauth.Identity, error) {
		authCalls.Add(1)
		if u != "example-bot" || p != "synthetic" {
			return botauth.Identity{}, botauth.ErrCredentials
		}
		return identity, nil
	}), Resolve: botauth.RepositoryResolver(bots), Ingress: ratelimit.New(600, 600), API: ratelimit.New(600, 600), Timeout: time.Second, Logger: logger}
	human := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) })
	}
	mux := http.NewServeMux()
	if err := handler.RegisterSyncRoutes(mux, "/custom/sync", repo, logger, human, gate); err != nil {
		t.Fatal(err)
	}
	service := chat.NewMessageService(repo, publishFunc(func(*model.WSMessage) bool { published.Add(1); return true }), nil)
	if err := handler.RegisterSendRoute(mux, "/custom/send", service, 168*time.Hour, human, gate); err != nil {
		t.Fatal(err)
	}
	var throttle atomic.Bool
	throttle.Store(true)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if throttle.Swap(false) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(429)
			return
		}
		if r.Method == "POST" && postCalls.Add(1) == 1 {
			result := httptest.NewRecorder()
			mux.ServeHTTP(result, r)
			if result.Code != 201 {
				t.Errorf("initial send: %d %s", result.Code, result.Body)
			}
			// Simulate a proxy losing the successful response after the server commits.
			w.Header().Set("Retry-After", "11")
			w.WriteHeader(503)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	makeBot := func() *bot {
		b := &bot{client: server.Client(), syncURL: server.URL + "/custom/sync", sendURL: server.URL + "/custom/send", user: "example-bot", password: "synthetic", selfID: principal.ID, statePath: statePath, now: time.Now}
		if err := b.load(); err != nil {
			t.Fatal(err)
		}
		return b
	}
	b := makeBot()
	var retry *retryError
	if err := b.poll(context.Background()); !errors.As(err, &retry) || retry.delay != 7*time.Second {
		t.Fatalf("429 delay: %v", err)
	}
	if err := b.poll(context.Background()); !errors.As(err, &retry) || retry.delay != 11*time.Second {
		t.Fatalf("503 delay: %v", err)
	}
	b = makeBot()
	if b.state.Pending == nil {
		t.Fatal("pending send not durable")
	}
	if err := b.poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if published.Load() != 1 || postCalls.Load() != 2 {
		t.Fatalf("recovery republished: %d posts %d", published.Load(), postCalls.Load())
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM messages WHERE text_content='@example-bot received'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("reply count %d: %v", count, err)
	}
	b = makeBot()
	if b.state.Pending != nil || b.state.Cursor.ID == "" {
		t.Fatal("cursor not durable")
	}
	if err := b.poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if postCalls.Load() != 2 {
		t.Fatal("own bot messages caused replies")
	}
	if authCalls.Load() < 5 {
		t.Fatal("pages/replays bypassed authentication")
	}
	// Old ambiguous sends must not be retried beyond the retention guarantee.
	b.state.Pending = &pending{Started: time.Now().Add(-168 * time.Hour)}
	if err := b.finishPending(context.Background()); err == nil {
		t.Fatal("expired pending send retried")
	}
}

func TestExampleRequiresSecureEndpoints(t *testing.T) {
	for _, bad := range []string{"http://example.home.arpa/api/messages", "https://u:p@example.home.arpa/api/messages", "https://example.home.arpa/api/messages?secret=x", "https://example.home.arpa/api/messages#fragment", "/api/messages"} {
		if endpoint(bad) {
			t.Fatalf("accepted %s", bad)
		}
	}
	if !endpoint("https://example.home.arpa/api/messages") {
		t.Fatal("HTTPS endpoint rejected")
	}
}

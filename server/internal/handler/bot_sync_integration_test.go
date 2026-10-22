package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/middleware"
	"meowgram/server/internal/model"
	"meowgram/server/internal/ratelimit"
	"meowgram/server/internal/repository"
	"meowgram/server/internal/testutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBotSyncPagingAndPolling(t *testing.T) {
	db, m, _ := testutil.Database(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := repository.NewMessageRepository(db)
	identity := botauth.Identity{Namespace: "sync", ExternalID: uuid.NewString(), Username: "bot"}
	principal, err := repository.NewBotRepository(db).Resolve(context.Background(), identity.Namespace, identity.ExternalID, identity.Username)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(authelia_sub,username) VALUES('history-human','human')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO messages(id,sender_id,bot_principal_id,text_content,created_at)
 SELECT ('10000000-0000-0000-0000-' || lpad(g::text,12,'0'))::uuid,
 CASE WHEN g=1000 THEN NULL ELSE 'history-human' END,
 CASE WHEN g=1000 THEN $1::uuid ELSE NULL END,'history '||g,'2020-01-02T00:00:00Z' FROM generate_series(1,1000) g`, principal.ID); err != nil {
		t.Fatal(err)
	}
	var revoked, unavailable atomic.Bool
	var checks atomic.Int32
	now := time.Now()
	clock := func() time.Time { return now }
	gate := &botauth.Gate{Directory: sendDirectoryFunc(func(_ context.Context, u, p string) (botauth.Identity, error) {
		checks.Add(1)
		if unavailable.Load() {
			return botauth.Identity{}, botauth.ErrUnavailable
		}
		if revoked.Load() {
			return botauth.Identity{}, botauth.ErrForbidden
		}
		if u != "bot" || p != "synthetic" {
			return botauth.Identity{}, botauth.ErrCredentials
		}
		return identity, nil
	}), Resolve: botauth.RepositoryResolver(repository.NewBotRepository(db)), Logger: logger, Timeout: 5 * time.Second,
		Ingress: ratelimit.NewWithClock(60, 4, clock), API: ratelimit.NewWithClock(30, 2, clock)}
	human := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) })
	}
	makeRoute := func() http.Handler {
		mux := http.NewServeMux()
		if err := RegisterSyncRoute(mux, "/api/messages/sync", repo, logger, human, gate); err != nil {
			t.Fatal(err)
		}
		if err := RegisterSendRoute(mux, "/api/messages", chat.NewMessageService(repo, sendPublishFunc(func(*model.WSMessage) bool { return true }), nil), 168*time.Hour, human, gate); err != nil {
			t.Fatal(err)
		}
		return middleware.GlobalRateLimit(ratelimit.NewWithClock(60, 4, clock), "/healthz")(mux)
	}
	route := makeRoute()
	request := func(method, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "https://example.home.arpa"+path, strings.NewReader(body))
		r.SetBasicAuth("bot", "synthetic")
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		route.ServeHTTP(w, r)
		return w
	}
	after := "2020-01-01T00:00:00+00:00"
	afterID := ""
	seen := map[string]bool{}
	lastID := ""
	for page := 0; page < 3; page++ {
		now = now.Add(time.Minute)
		// First request deliberately leaves + unencoded; subsequent cursor is encoded.
		path := "/api/messages/sync?after=" + after + "&after_id=" + afterID
		w := request("GET", path, "", "")
		if w.Code != 200 {
			t.Fatalf("page %d: %d %s", page, w.Code, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("sync cached")
		}
		var messages []model.Message
		if err := json.Unmarshal(w.Body.Bytes(), &messages); err != nil {
			t.Fatal(err)
		}
		if page < 2 && (len(messages) != 500 || w.Header().Get("X-Has-More") != "true") {
			t.Fatalf("page shape %d", len(messages))
		}
		if page == 2 && (len(messages) != 0 || strings.TrimSpace(w.Body.String()) != "[]") {
			t.Fatal("empty page not array")
		}
		for _, msg := range messages {
			if seen[msg.ID] || msg.ID <= lastID || msg.SenderID == "" || msg.Username == "" || msg.SenderKind == "" || msg.TextContent == "" || msg.CreatedAt.IsZero() {
				t.Fatalf("lost/duplicate/unsorted message %+v", msg)
			}
			seen[msg.ID] = true
			lastID = msg.ID
			after = url.QueryEscape(msg.CreatedAt.UTC().Format(time.RFC3339Nano))
			afterID = msg.ID
		}
		if page == 0 {
			now = now.Add(time.Minute)
			revoked.Store(true)
			if w := request("GET", "/api/messages/sync?after="+after+"&after_id="+afterID, "", ""); w.Code != 403 {
				t.Fatal("next page bypassed group check")
			}
			revoked.Store(false)
			route = makeRoute() // process/route restart retains cursor semantics
		}
	}
	if len(seen) != 1000 || checks.Load() != 4 {
		t.Fatalf("rows %d checks %d", len(seen), checks.Load())
	}
	now = now.Add(time.Minute)
	unavailable.Store(true)
	w := request("GET", "/api/messages/sync?after="+after+"&after_id="+afterID, "", "")
	if w.Code != 503 || w.Header().Get("Retry-After") == "" {
		t.Fatal("directory failure not backed off")
	}
	unavailable.Store(false)
	// Six minutes at one poll/3 seconds plus one reply/6 seconds = 30 API requests/min.
	now = now.Add(time.Minute)
	for i := 0; i < 120; i++ {
		now = now.Add(3 * time.Second)
		w := request("GET", "/api/messages/sync?after=2100-01-01T00:00:00Z", "", "")
		if w.Code != 200 {
			t.Fatalf("poll %d throttled: %d", i, w.Code)
		}
		if i%2 == 0 {
			w = request("POST", "/api/messages", `{"text_content":"poll reply"}`, fmt.Sprintf("pourover-reply-%d", i))
			if w.Code != 201 {
				t.Fatalf("reply %d throttled: %d", i, w.Code)
			}
		}
	}
}

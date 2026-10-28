package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/middleware"
	"meowgram/server/internal/model"
	"meowgram/server/internal/ratelimit"
	"meowgram/server/internal/repository"
	"meowgram/server/internal/testutil"
)

type scopedBotFixture struct {
	db                      *sql.DB
	conversations           *repository.ConversationRepository
	channel, bot            string
	route                   http.Handler
	authCalls, publications atomic.Int32
}

// Directory is a counting gate stub here; the dedicated real-binary harness
// separately proves verified LDAPS. Persistence and membership use real PostgreSQL.
func newScopedBotFixture(t *testing.T, now func() time.Time, apiRate, apiBurst, ingressRate, ingressBurst int) *scopedBotFixture {
	t.Helper()
	db, migration, _ := testutil.Database(t)
	if err := migration.Up(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO users(authelia_sub,username) VALUES('scoped-human','Human')"); err != nil {
		t.Fatal(err)
	}
	f := &scopedBotFixture{db: db, conversations: repository.NewConversationRepository(db)}
	identity := botauth.Identity{Namespace: "scoped-fixture", ExternalID: uuid.NewString(), Username: "scoped-bot"}
	bots := repository.NewBotRepository(db)
	bot, err := bots.Resolve(context.Background(), identity.Namespace, identity.ExternalID, identity.Username)
	if err != nil {
		t.Fatal(err)
	}
	f.bot = bot.ID
	conversation, _, err := f.conversations.Create(context.Background(), "scoped-human", nil, []repository.PrincipalRef{{Kind: "bot", ID: bot.ID}}, true)
	if err != nil {
		t.Fatal(err)
	}
	f.channel = *conversation.ChannelID
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gate := &botauth.Gate{
		Directory: sendDirectoryFunc(func(context.Context, string, string) (botauth.Identity, error) {
			f.authCalls.Add(1)
			return identity, nil
		}),
		Resolve: botauth.RepositoryResolver(bots), Logger: logger, Timeout: 5 * time.Second,
		Ingress: ratelimit.NewWithClock(ingressRate, ingressBurst, now),
		API:     ratelimit.NewWithClock(apiRate, apiBurst, now),
	}
	human := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(401) })
	}
	repo := repository.NewMessageRepository(db)
	publisher := sendPublishFunc(func(m *model.WSMessage) bool {
		if m.ChannelID == nil || *m.ChannelID != f.channel {
			t.Error("publication lost private channel")
		}
		f.publications.Add(1)
		return true
	})
	// Ten replies per minute, even with the smallest useful write burst.
	service := chat.NewMessageService(repo, publisher, ratelimit.NewWithClock(10, 1, now))
	mux := http.NewServeMux()
	if err := RegisterSendRoute(mux, "/api/messages", service, 168*time.Hour, human, gate); err != nil {
		t.Fatal(err)
	}
	if err := RegisterSyncRoute(mux, "/api/messages/sync", repo, logger, human, gate); err != nil {
		t.Fatal(err)
	}
	f.route = middleware.GlobalRateLimit(ratelimit.NewWithClock(ingressRate, ingressBurst, now), "/health")(mux)
	return f
}
func (f *scopedBotFixture) request(method, path, key, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://example.home.arpa"+path, strings.NewReader(body))
	r.SetBasicAuth("scoped-bot", "synthetic")
	if method == "POST" {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	f.route.ServeHTTP(w, r)
	return w
}
func TestMemberBotBudgets(t *testing.T) {
	var seconds atomic.Int64
	now := func() time.Time { return time.Unix(seconds.Load(), 0) }
	// Approved minimum API=30/min and ingress=60/min, not permissive defaults.
	f := newScopedBotFixture(t, now, 30, 2, 60, 2)
	for minute := 0; minute < 6; minute++ {
		for poll := 0; poll < 20; poll++ {
			seconds.Store(int64(minute*60 + poll*3))
			w := f.request("GET", "/api/messages/sync?scope=member&after=0", "", "")
			if w.Code != 200 || w.Header().Get("X-Membership-Version") == "" {
				t.Fatalf("minute %d poll %d: status=%d", minute, poll, w.Code)
			}
			var rows []model.Message
			if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != minute*10+(poll+1)/2 {
				t.Fatalf("merged page lost accepted replies: %d", len(rows))
			}
			for _, row := range rows {
				if row.ChannelID == nil || *row.ChannelID != f.channel {
					t.Fatal("member stream lost source channel")
				}
			}
			if poll%2 == 0 {
				body := fmt.Sprintf("{\"text_content\":\"reply\",\"channel_id\":%q}", f.channel)
				w = f.request("POST", "/api/messages", fmt.Sprintf("reply-%d-%d", minute, poll), body)
				if w.Code != 201 {
					t.Fatalf("minute %d reply %d: status=%d", minute, poll, w.Code)
				}
			}
		}
	}
	if f.authCalls.Load() != 180 || f.publications.Load() != 60 {
		t.Fatalf("auth=%d publications=%d", f.authCalls.Load(), f.publications.Load())
	}
	var rows int
	if err := f.db.QueryRow("SELECT count(*) FROM messages WHERE channel_id=$1", f.channel).Scan(&rows); err != nil || rows != 60 {
		t.Fatalf("persisted=%d err=%v", rows, err)
	}
	// Positive rate-limit control: consume the current API/ingress burst.
	denied := false
	for i := 0; i < 8; i++ {
		w := f.request("GET", "/api/messages/sync?scope=member&after=0", "", "")
		if w.Code == 429 {
			if w.Header().Get("Retry-After") == "" {
				t.Fatal("missing retry delay")
			}
			denied = true
			break
		}
	}
	if !denied {
		t.Fatal("fixture failed to exercise rate limiting")
	}
}
func TestScopedHTTPConcurrentPublicationAndRevokedReplay(t *testing.T) {
	f := newScopedBotFixture(t, time.Now, 1000, 1000, 2000, 2000)
	body := fmt.Sprintf("{\"text_content\":\"once\",\"channel_id\":%q}", f.channel)
	const n = 24
	results := make(chan *httptest.ResponseRecorder, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- f.request("POST", "/api/messages", "same", body) }()
	}
	close(start)
	wg.Wait()
	close(results)
	exact := ""
	for w := range results {
		if w.Code != 201 {
			t.Fatalf("duplicate status=%d", w.Code)
		}
		if exact == "" {
			exact = w.Body.String()
		} else if exact != w.Body.String() {
			t.Fatal("duplicate response bytes changed")
		}
	}
	if f.publications.Load() != 1 || f.authCalls.Load() != n {
		t.Fatalf("publications=%d auth=%d", f.publications.Load(), f.authCalls.Load())
	}
	var count int
	if err := f.db.QueryRow("SELECT count(*) FROM messages WHERE channel_id=$1", f.channel).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rows=%d err=%v", count, err)
	}
	if err := f.conversations.ChangeMember(context.Background(), "scoped-human", f.channel, repository.PrincipalRef{Kind: "bot", ID: f.bot}, false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path, key, body string }{
		{"POST", "/api/messages", "same", body},
		{"POST", "/api/messages", "same", "{\"text_content\":\"different\"}"},
		{"GET", "/api/messages/sync?after=0&channel_id=" + f.channel, "", ""},
	} {
		if w := f.request(tc.method, tc.path, tc.key, tc.body); w.Code != 403 {
			t.Fatalf("revoked access/replay status=%d", w.Code)
		}
	}
	if f.publications.Load() != 1 || f.authCalls.Load() != n+3 {
		t.Fatal("revoked requests bypassed fresh authentication or published")
	}
}

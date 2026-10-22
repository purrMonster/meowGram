package botauth_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"meowgram/server/internal/botauth"
	"meowgram/server/internal/ratelimit"
	"meowgram/server/internal/repository"
)

func TestGateStableUsernameAndRecreatedPrincipal(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("scratch PostgreSQL required")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/verification" {
		t.Skip("TEST_DATABASE_URL database must be /verification")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := "username_probe_" + uuid.New().String()[:8]
	if _, err = admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(`DROP DATABASE ` + name); err != nil {
			t.Error(err)
		}
	}()
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := os.Getenv("MIGRATIONS_PATH")
	if root == "" {
		root = "../../migrations"
	}
	root, err = filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.New((&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	// Same visible username is permitted for a human and a bot.
	if _, err := db.Exec(`INSERT INTO users(authelia_sub, username) VALUES('human-sub', 'bot')`); err != nil {
		t.Fatal(err)
	}
	identity := botauth.Identity{Namespace: "test", ExternalID: uuid.NewString(), Username: "bot"}
	var got botauth.Principal
	gate := botauth.Gate{
		SyncPath: "/sync", SendPath: "/send", Timeout: time.Second,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Ingress: ratelimit.New(60, 10), API: ratelimit.New(60, 10),
		Directory: authFunc(func(context.Context, string, string) (botauth.Identity, error) { return identity, nil }),
		Resolve:   botauth.RepositoryResolver(repository.NewBotRepository(db)),
		HumanOnly: func(next http.Handler) http.Handler { return next },
	}
	handler, err := gate.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		got, ok = botauth.PrincipalFromContext(r.Context())
		if !ok {
			t.Error("missing principal")
		}
		w.WriteHeader(204)
	}))
	if err != nil {
		t.Fatal(err)
	}
	request := func() botauth.Principal {
		t.Helper()
		r := httptest.NewRequest("GET", "https://example.com/sync", nil)
		r.SetBasicAuth("BOT", "synthetic-password")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatalf("gate status %d", w.Code)
		}
		return got
	}
	first := request()
	if first.Username != "bot" {
		t.Fatal("did not provision verified uid")
	}
	// Even a later differing resolver input must not rewrite an existing profile.
	identity.Username = "changed-label"
	if again := request(); again != first {
		t.Fatal("resolve rewrote existing username")
	}
	identity.Username = "bot"
	identity.ExternalID = uuid.NewString()
	second := request()
	if second.ID == first.ID || second.Username != "bot" {
		t.Fatal("recreated login reused old principal")
	}
	messages := repository.NewMessageRepository(db)
	bot, err := messages.CreateBot(context.Background(), first.ID, "bot message")
	if err != nil {
		t.Fatal(err)
	}
	human, err := messages.Create(context.Background(), "human-sub", "bot", "human message")
	if err != nil {
		t.Fatal(err)
	}
	if bot.Username != "bot" || human.Username != "bot" || bot.SenderKind != "bot" || human.SenderKind != "human" || bot.SenderID == human.SenderID {
		t.Fatal("human/bot identity or username changed")
	}
	var humanName string
	if err := db.QueryRow(`SELECT username FROM users WHERE authelia_sub='human-sub'`).Scan(&humanName); err != nil || humanName != "bot" {
		t.Fatalf("human changed: %v", err)
	}
}

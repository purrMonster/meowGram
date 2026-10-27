package repository

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestBotIdentityMigrationAndLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("scratch PostgreSQL required")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/verification" {
		t.Skip("requires disposable PostgreSQL: TEST_DATABASE_URL database must be /verification")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := "bot_probe_" + uuid.New().String()[:8]
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
	apply := func(file string) {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(content)); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}
	for _, file := range []string{"000001_create_users_table.up.sql", "000002_create_messages_table.up.sql", "000003_remove_duplicate_user_index.up.sql", "000004_push_devices.up.sql"} {
		apply(file)
	}
	ctx := context.Background()
	const oldID = "10000000-0000-0000-0000-000000000001"
	const oldAt = "2020-01-01T00:00:00Z"
	if _, err := db.Exec(`INSERT INTO users(authelia_sub,username) VALUES('legacy','original'); INSERT INTO messages(id,sender_id,text_content,created_at) VALUES('` + oldID + `','legacy','preserved','` + oldAt + `')`); err != nil {
		t.Fatal(err)
	}
	apply("000005_bot_principals.up.sql")
	var id, sub, body string
	var at time.Time
	if err := db.QueryRow(`SELECT id,sender_id,text_content,created_at FROM messages WHERE id=$1`, oldID).Scan(&id, &sub, &body, &at); err != nil {
		t.Fatal(err)
	}
	if id != oldID || sub != "legacy" || body != "preserved" || at.UTC().Format(time.RFC3339) != oldAt {
		t.Fatal("migration changed legacy message")
	}
	repo := NewBotRepository(db)
	external := uuid.NewString()
	ids := make(chan string, 20)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := repo.Resolve(ctx, "directory-a", external, "bot")
			if err != nil {
				t.Error(err)
				return
			}
			ids <- p.ID
		}()
	}
	wg.Wait()
	close(ids)
	var principal string
	for id := range ids {
		if principal != "" && id != principal {
			t.Fatal("duplicate mapping")
		}
		principal = id
	}
	if principal == "" {
		t.Fatal("no principal")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM bot_principals`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("orphan principals: %d %v", count, err)
	}
	renamed, err := repo.Resolve(ctx, "directory-a", external, "Renamed 🐾")
	if err != nil || renamed.ID != principal || renamed.Username != "bot" {
		t.Fatalf("rename changed identity: %v", err)
	}
	recreated, err := repo.Resolve(ctx, "directory-a", uuid.NewString(), "Renamed 🐾")
	if err != nil || recreated.ID == principal {
		t.Fatalf("recreation reused identity: %v", err)
	}
	other, err := repo.Resolve(ctx, "directory-b", external, "bot")
	if err != nil || other.ID == principal {
		t.Fatalf("namespace collision: %v", err)
	}
	// Deliberately collide the exact wire ID with a valid human OIDC subject.
	if _, err := db.Exec(`INSERT INTO users(authelia_sub,username) VALUES($1,'human')`, principal); err != nil {
		t.Fatal(err)
	}
	messages := NewMessageRepository(db)
	human, err := messages.Create(ctx, principal, "human", "human message")
	if err != nil {
		t.Fatal(err)
	}
	bot, err := messages.CreateBot(ctx, principal, "bot message")
	if err != nil {
		t.Fatal(err)
	}
	if human.SenderID != bot.SenderID || human.SenderKind != "human" || bot.SenderKind != "bot" {
		t.Fatal("sender kinds lost")
	}
	if _, err := db.Exec(`INSERT INTO messages(sender_id,bot_principal_id,text_content) VALUES($1,$1,'invalid')`, principal); err == nil {
		t.Fatal("accepted two senders")
	}
	if _, err := db.Exec(`INSERT INTO messages(text_content) VALUES('invalid')`); err == nil {
		t.Fatal("accepted missing sender")
	}
	if _, err := db.Exec(`DELETE FROM bot_principals WHERE id=$1`, principal); err == nil {
		t.Fatal("deleted principal with references")
	}
	// Removing a mapping does not erase the profile or its history.
	if _, err := db.Exec(`DELETE FROM bot_directory_identities WHERE principal_id=$1`, principal); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM bot_principals WHERE id=$1`, principal); err == nil {
		t.Fatal("deleted principal with messages")
	}
	apply("000005_bot_principals.down.sql")
	// This historical schema-5 boundary predates channel_id. Keep its read
	// oracle frozen to v2.0.0; current runtime writes and lifecycle assertions remain above.
	historicalReads := &frozenR20MessageRepository{db: db}
	for _, recent := range []bool{false, true} {
		var gotKind string
		rows, err := historicalReads.GetMessagesAfter(ctx, time.Time{}, "", 500)
		if recent {
			rows, err = historicalReads.GetRecent(ctx, 50)
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range rows {
			if m.ID == bot.ID {
				gotKind = m.SenderKind
				if m.Username != "bot" {
					t.Fatal("bot label lost")
				}
			}
		}
		if gotKind != "bot" || len(rows) != 3 {
			t.Fatal("history not preserved")
		}
	}
}

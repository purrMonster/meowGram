package repository

import (
	"context"
	"meowgram/server/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIdempotencyMigrationRollbackAndPartialReapply(t *testing.T) {
	// This regression proves the 5 -> 6 boundary, even after new schemas ship.
	db, m, root := testutil.DatabaseThrough(t, 6)
	if err := m.Migrate(5); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(authelia_sub,username) VALUES('legacy','human'); INSERT INTO messages(sender_id,text_content) VALUES('legacy','legacy message')`); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := db.QueryRow(`SELECT row_to_json(m)::text FROM messages m`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile(filepath.Join(root, "000006_message_idempotency.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate interruption after table creation and before index creation.
	first, _, _ := strings.Cut(string(up), "CREATE INDEX")
	if _, err := db.Exec(first); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	p := SendPrincipal{Kind: "human", ID: "legacy", Username: "human"}
	repo := NewMessageRepository(db)
	sent, err := repo.CreateIdempotent(context.Background(), p, "key", "hello", "hello", 168*time.Hour, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(string(up)); err != nil {
			t.Fatal(err)
		}
	}
	version, dirty, err := m.Version()
	if err != nil || dirty || version != 6 {
		t.Fatalf("version %d dirty %v err %v", version, dirty, err)
	}
	var after string
	if err := db.QueryRow(`SELECT row_to_json(m)::text FROM messages m WHERE text_content='legacy message'`).Scan(&after); err != nil || before != after {
		t.Fatalf("legacy changed %v", err)
	}
	replay, err := repo.CreateIdempotent(context.Background(), p, "key", "hello", "hello", 168*time.Hour, func() error { t.Fatal("replay charged write budget"); return nil })
	if err != nil || !replay.Replay || string(replay.Body) != string(sent.Body) {
		t.Fatalf("replay changed %v", err)
	}
	var indexes int
	if err := db.QueryRow(`SELECT count(*) FROM pg_indexes WHERE tablename='message_idempotency' AND indexname='idx_message_idempotency_expiry'`).Scan(&indexes); err != nil || indexes != 1 {
		t.Fatalf("index count %d %v", indexes, err)
	}
}

func TestIdempotencyExpiryAndTransactionFailure(t *testing.T) {
	db, m, _ := testutil.Database(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(authelia_sub,username) VALUES('human','human')`); err != nil {
		t.Fatal(err)
	}
	repo := NewMessageRepository(db)
	p := SendPrincipal{Kind: "human", ID: "human", Username: "human"}
	send := func() (*SendResult, error) {
		return repo.CreateIdempotent(context.Background(), p, "key", "hello", "hello", 168*time.Hour, func() error { return nil })
	}
	one, err := send()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE message_idempotency SET expires_at=clock_timestamp()`); err != nil {
		t.Fatal(err)
	}
	two, err := send()
	if err != nil || two.Replay || one.Message.ID == two.Message.ID {
		t.Fatalf("expiry did not create new operation %v", err)
	}
	// Fail after message insertion but before the idempotency row can commit.
	if _, err := db.Exec(`ALTER TABLE message_idempotency ADD CONSTRAINT reject_new_keys CHECK (operation_key='key')`); err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateIdempotent(context.Background(), p, "fail", "rollback", "rollback", 168*time.Hour, func() error { return nil })
	if err == nil {
		t.Fatal("expected transaction failure")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM messages WHERE text_content='rollback'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed transaction wrote message: %d %v", count, err)
	}
}

func TestIdempotencyBoundedExpiredCleanup(t *testing.T) {
	db, m, _ := testutil.Database(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(authelia_sub,username) VALUES('cleanup','human')`); err != nil {
		t.Fatal(err)
	}
	repo := NewMessageRepository(db)
	p := SendPrincipal{Kind: "human", ID: "cleanup", Username: "human"}
	send := func() (*SendResult, error) {
		return repo.CreateIdempotent(context.Background(), p, "active", "hello", "hello", 168*time.Hour, func() error { return nil })
	}
	first, err := send()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO message_idempotency SELECT sender_kind,sender_id,'expired-'||g,fingerprint,message_id,response_body,clock_timestamp()-interval '1 hour' FROM message_idempotency CROSS JOIN generate_series(1,205) g WHERE operation_key='active'`); err != nil {
		t.Fatal(err)
	}
	check := func(want int) {
		t.Helper()
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM message_idempotency WHERE expires_at <= clock_timestamp()`).Scan(&count); err != nil || count != want {
			t.Fatalf("expired count %d want %d: %v", count, want, err)
		}
	}
	// A failed operation cannot perform cleanup or persist a message.
	if _, err := db.Exec(`ALTER TABLE message_idempotency ADD CONSTRAINT fail_cleanup CHECK(operation_key <> 'fail')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateIdempotent(context.Background(), p, "fail", "x", "x", 168*time.Hour, func() error { return nil }); err == nil {
		t.Fatal("expected failure")
	}
	check(205)
	// New writes and replays each commit at most 100 deletions.
	if _, err := repo.CreateIdempotent(context.Background(), p, "new", "new", "new", 168*time.Hour, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	check(105)
	replay, err := send()
	if err != nil || !replay.Replay || string(replay.Body) != string(first.Body) {
		t.Fatalf("replay changed: %v", err)
	}
	check(5)
	if _, err := send(); err != nil {
		t.Fatal(err)
	}
	check(0)
	var messages, active int
	if err := db.QueryRow(`SELECT count(*) FROM messages`).Scan(&messages); err != nil || messages != 2 {
		t.Fatalf("cleanup changed messages: %d %v", messages, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM message_idempotency`).Scan(&active); err != nil || active != 2 {
		t.Fatalf("cleanup changed active keys: %d %v", active, err)
	}
}

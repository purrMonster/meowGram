package repository

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"meowgram/server/internal/database"
)

// TEST_DATABASE_URL is supplied only by the disposable verification stack.
func TestPostgresPaginationAtEqualTimestamp(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("run deploy/scripts/verify.ps1 for PostgreSQL integration")
	}
	db, err := database.Open(url, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `INSERT INTO users (authelia_sub, username) VALUES ('scratch-pagination', 'scratch') ON CONFLICT (authelia_sub) DO NOTHING`)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	// All 1001 rows share a timestamp, straddling two full pages.
	for i := 1; i <= 1001; i++ {
		id := fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
		_, err = db.ExecContext(ctx, `INSERT INTO messages (id, sender_id, text_content, created_at) VALUES ($1, 'scratch-pagination', 'test', $2) ON CONFLICT (id) DO NOTHING`, id, at)
		if err != nil {
			t.Fatal(err)
		}
	}
	repo := NewMessageRepository(db)
	cursor := ""
	total := 0
	for page := 0; page < 4; page++ {
		messages, err := repo.GetMessagesAfter(ctx, at, cursor, 500)
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range messages {
			if message.ID <= cursor {
				t.Fatal("duplicate or unordered cursor")
			}
			cursor = message.ID
			total++
		}
		if len(messages) < 500 {
			break
		}
	}
	if total != 1001 {
		t.Fatalf("got %d messages; want 1001", total)
	}
	// Removing the redundant index must not remove identity uniqueness.
	var exists bool
	err = db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_users_authelia_sub')`).Scan(&exists)
	if err != nil || !exists {
		t.Fatalf("identity unique constraint missing: %v", err)
	}
}

func TestPostgresDeviceOwnershipAndExpiry(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("scratch PostgreSQL required")
	}
	db, err := database.Open(url, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	for _, sub := range []string{"device-user-a", "device-user-b"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO users (authelia_sub,username) VALUES ($1,'scratch') ON CONFLICT (authelia_sub) DO NOTHING`, sub); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewDeviceRepository(db)
	if err := repo.Register(ctx, "device-user-a", "scratch-device-active", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Register(ctx, "device-user-a", "scratch-device-expired", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Unregister(ctx, "device-user-b", "scratch-device-active"); err != nil {
		t.Fatal(err)
	}
	devices, err := repo.ActiveAfter(ctx, 0)
	if err != nil || len(devices) != 1 || devices[0].Token != "scratch-device-active" {
		t.Fatalf("expiry or owner isolation failed: count=%d err=%v", len(devices), err)
	}
	if err := repo.Register(ctx, "device-user-b", "scratch-device-active", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Unregister(ctx, "device-user-a", "scratch-device-active"); err != nil {
		t.Fatal(err)
	}
	devices, err = repo.ActiveAfter(ctx, 0)
	if err != nil || len(devices) != 1 {
		t.Fatal("old owner revoked transferred registration")
	}
	if err := repo.Unregister(ctx, "device-user-b", "scratch-device-active"); err != nil {
		t.Fatal(err)
	}
	devices, err = repo.ActiveAfter(ctx, 0)
	if err != nil || len(devices) != 0 {
		t.Fatal("logout did not revoke registration")
	}
}

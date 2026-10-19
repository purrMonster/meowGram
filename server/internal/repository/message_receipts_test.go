package repository

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"meowgram/server/internal/database"
	"os"
	"sync"
	"testing"
)

func TestDurableReceiptConcurrencyConflictAndRestart(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("scratch PostgreSQL required")
	}
	db, err := database.Open(dsn, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	sender := uuid.NewString()
	key := uuid.NewString()
	if _, err = db.Exec(`INSERT INTO bot_principals(id,username) VALUES($1,'synthetic bot')`, sender); err != nil {
		t.Fatal(err)
	}
	repo := NewMessageRepository(db)
	var wg sync.WaitGroup
	ids := make(chan string, 16)
	fresh := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg, replay, err := repo.CreateOnce(ctx, "bot", sender, "bot", key, "synthetic notification")
			if err != nil {
				t.Error(err)
				return
			}
			ids <- msg.ID
			fresh <- !replay
		}()
	}
	wg.Wait()
	close(ids)
	close(fresh)
	first := ""
	for id := range ids {
		if first != "" && first != id {
			t.Fatal("duplicate messages")
		}
		first = id
	}
	count := 0
	for created := range fresh {
		if created {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("fresh=%d", count)
	}
	_, _, err = repo.CreateOnce(ctx, "bot", sender, "bot", key, "changed")
	if !errors.Is(err, ErrReceiptConflict) {
		t.Fatal("key conflict accepted")
	}
	restarted := NewMessageRepository(db)
	msg, replay, err := restarted.CreateOnce(ctx, "bot", sender, "renamed", key, "synthetic notification")
	if err != nil || !replay || msg.ID != first {
		t.Fatal("lost replay after restart")
	}
	if _, _, err = repo.CreateOnce(ctx, "bot", uuid.NewString(), "missing", uuid.NewString(), "fails FK"); err == nil {
		t.Fatal("invalid sender persisted")
	}
}

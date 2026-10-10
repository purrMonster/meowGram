package auth

import (
	"crypto/sha256"
	"meowgram/server/internal/model"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWSTicketSingleUseConcurrent(t *testing.T) {
	store := NewWSTicketStore()
	user := &model.User{ID: "id", AutheliaSub: "subject", Username: "cat"}
	ticket, err := store.Issue(user)
	if err != nil {
		t.Fatal(err)
	}
	user.Username = "changed"
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, ok := store.Consume(ticket)
			if ok {
				successes.Add(1)
				if got.Username != "cat" {
					t.Error("ticket identity was mutated")
				}
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("consumed %d times", successes.Load())
	}
}

func TestWSTicketExpiryAndInvalidInput(t *testing.T) {
	store := NewWSTicketStore()
	if _, err := store.Issue(nil); err == nil {
		t.Fatal("nil identity accepted")
	}
	for _, value := range []string{"", "unknown"} {
		if _, ok := store.Consume(value); ok {
			t.Fatal("invalid ticket accepted")
		}
	}
	token, err := store.Issue(&model.User{ID: "id"})
	if err != nil {
		t.Fatal(err)
	}
	key := sha256.Sum256([]byte(token))
	entry := store.tickets[key]
	entry.expiresAt = time.Now().Add(-time.Second)
	store.tickets[key] = entry
	if _, ok := store.Consume(token); ok {
		t.Fatal("expired ticket accepted")
	}
	if len(store.tickets) != 0 {
		t.Fatal("expired ticket retained")
	}
}

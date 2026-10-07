package chat

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"meowgram/server/internal/model"
)

type fakePublisher struct {
	mu    sync.Mutex
	calls []map[string]string
	title string
	body  string
}

func (f *fakePublisher) PublishToTopic(_ context.Context, topic, title, body string, data map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, data)
	f.title, f.body = title, body
	return nil
}

func (f *fakePublisher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func newTestClient(h *Hub, sub string, buf int) *Client {
	return &Client{
		hub:  h,
		send: make(chan *model.WSMessage, buf),
		user: &model.User{AutheliaSub: sub, Username: sub},
	}
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// A slow client evicted by the Hub must not cause a panic when frames are later
// addressed to it (previously ReadPump sent on the already-closed channel).
func TestHub_SendToEvictedClientDoesNotPanic(t *testing.T) {
	hub := NewHub(quietLogger(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	slow := newTestClient(hub, "slow", 1)
	if !hub.RegisterClient(slow) {
		t.Fatal("register failed")
	}
	// Overflow the 1-slot buffer so the Hub evicts and closes slow.send.
	for i := 0; i < 5; i++ {
		hub.Publish(&model.WSMessage{Type: "chat", TextContent: "x", CreatedAt: time.Now()})
	}
	// These used to panic with "send on closed channel".
	hub.SendTo(slow, &model.WSMessage{Type: "error", TextContent: "late"})
	hub.UnregisterClient(slow)

	// Hub must still be alive and serving.
	ok := newTestClient(hub, "ok", 16)
	if !hub.RegisterClient(ok) {
		t.Fatal("hub stopped unexpectedly")
	}
}

// After shutdown, client goroutines must not block forever on Hub channels.
func TestHub_OperationsAfterShutdownDoNotBlock(t *testing.T) {
	hub := NewHub(quietLogger(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	c := newTestClient(hub, "c", 16)
	hub.RegisterClient(c)
	cancel()
	<-hub.Done()

	done := make(chan struct{})
	go func() {
		hub.UnregisterClient(c)
		if hub.Publish(&model.WSMessage{Type: "chat"}) {
			t.Error("Publish should report false after shutdown")
		}
		if hub.RegisterClient(newTestClient(hub, "late", 1)) {
			t.Error("RegisterClient should report false after shutdown")
		}
		hub.SendTo(c, &model.WSMessage{Type: "error"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("hub operations blocked after shutdown")
	}
}

// Pushes must not carry message content or sender identity, and bursts coalesce.
func TestHub_PushIsContentFreeAndCoalesced(t *testing.T) {
	pub := &fakePublisher{}
	hub := NewHub(quietLogger(), pub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	for i := 0; i < 20; i++ {
		hub.Publish(&model.WSMessage{
			Type: "chat", ID: "id", SenderID: "secret-sub", Username: "whiskers",
			TextContent: "the secret plan", CreatedAt: time.Now(),
		})
	}

	deadline := time.Now().Add(2 * time.Second)
	for pub.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	if n := pub.count(); n != 1 {
		t.Fatalf("expected 1 coalesced push for a burst, got %d", n)
	}
	pub.mu.Lock()
	defer pub.mu.Unlock()
	all := pub.title + pub.body
	for _, v := range pub.calls[0] {
		all += v
	}
	for _, leak := range []string{"secret plan", "secret-sub", "whiskers"} {
		if strings.Contains(all, leak) {
			t.Errorf("push payload leaks %q", leak)
		}
	}
}

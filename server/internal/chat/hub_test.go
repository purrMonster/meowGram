package chat

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"meowgram/server/internal/model"
)

func TestHubLifecycleAndBroadcast(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	// Create test client instances with dummy users
	user1 := &model.User{ID: "usr-1", AutheliaSub: "sub-1", Username: "mittens"}
	user2 := &model.User{ID: "usr-2", AutheliaSub: "sub-2", Username: "whiskers"}

	client1 := &Client{
		hub:  hub,
		send: make(chan *model.WSMessage, sendBufferSize),
		user: user1,
	}
	client2 := &Client{
		hub:  hub,
		send: make(chan *model.WSMessage, sendBufferSize),
		user: user2,
	}

	// 1. Register Client 1
	hub.Register <- client1
	time.Sleep(50 * time.Millisecond)

	// Client 1 should receive the arrival system notice and initial presence roster
	select {
	case notice := <-client1.send:
		if notice.Type != "system" {
			t.Errorf("expected system notice on registration, got %s", notice.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for arrival notice on client 1")
	}

	select {
	case pres := <-client1.send:
		if pres.Type != "presence" {
			t.Errorf("expected presence roster on registration, got %s", pres.Type)
		}
		if len(pres.Users) != 1 {
			t.Errorf("expected 1 user in presence roster, got %d", len(pres.Users))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for presence roster on client 1")
	}

	// 2. Register Client 2
	hub.Register <- client2
	time.Sleep(50 * time.Millisecond)

	// Helper to drain pending registration / presence notices before testing broadcast
	drain := func(c *Client) {
		for {
			select {
			case <-c.send:
			default:
				return
			}
		}
	}
	drain(client1)
	drain(client2)

	// 3. Broadcast chat message from client 1
	testMsg := &model.WSMessage{
		Type:        "chat",
		ID:          "msg-123",
		SenderID:    user1.AutheliaSub,
		Username:    user1.Username,
		TextContent: "Meow world! 🐾",
		CreatedAt:   time.Now().UTC(),
	}

	hub.Broadcast <- testMsg

	// Both client 1 and client 2 should receive the broadcasted message
	for i, c := range []*Client{client1, client2} {
		select {
		case msg := <-c.send:
			if msg.Type != "chat" {
				t.Errorf("client %d expected type chat, got %s", i+1, msg.Type)
			}
			if msg.TextContent != "Meow world! 🐾" {
				t.Errorf("client %d expected text 'Meow world! 🐾', got %s", i+1, msg.TextContent)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("client %d timed out waiting for broadcast", i+1)
		}
	}

	// 4. Unregister Client 2
	hub.Unregister <- client2
	time.Sleep(50 * time.Millisecond)

	// Client 1 should receive departure notice and updated presence roster
	select {
	case notice := <-client1.send:
		if notice.Type != "system" {
			t.Errorf("expected system notice on unregister, got %s", notice.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for departure notice on client 1")
	}

	select {
	case pres := <-client1.send:
		if pres.Type != "presence" {
			t.Errorf("expected presence roster on unregister, got %s", pres.Type)
		}
		if len(pres.Users) != 1 {
			t.Errorf("expected 1 remaining user in presence roster, got %d", len(pres.Users))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for updated presence roster on client 1")
	}
}

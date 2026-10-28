package chat

import (
	"context"
	"meowgram/server/internal/model"
	"testing"
	"time"
)

func TestLegacyHubExcludesGroupWithLoungePositiveControl(t *testing.T) {
	pub := &fakePublisher{}
	hub := NewHub(quietLogger(), pub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	client := newTestClient(hub, "viewer", 32)
	if !hub.RegisterClient(client) {
		t.Fatal("register")
	}
	channel := "10000000-0000-0000-0000-000000000001"
	hub.Publish(&model.WSMessage{Type: "chat", ID: "private", ChannelID: &channel})
	// Same broadcast queue is FIFO: observing this control proves the private
	// frame was processed, without relying on a sleep to infer non-delivery.
	hub.Publish(&model.WSMessage{Type: "chat", ID: "lounge"})
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case m := <-client.send:
			if m.ID == "private" {
				t.Fatal("private group leaked into legacy socket")
			}
			if m.ID == "lounge" {
				goto delivered
			}
		case <-timeout.C:
			t.Fatal("lounge positive control missing")
		}
	}
delivered:
	deadline := time.Now().Add(2 * time.Second)
	for pub.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if pub.count() != 1 {
		t.Fatal("lounge push positive control missing")
	}
	cancel()
	<-hub.Done()
}

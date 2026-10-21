package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"meowgram/server/internal/model"
	"meowgram/server/internal/ratelimit"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadPumpRejectsBeforeDatabaseWrite(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	limiter.Allow("human:limited")
	hub := NewHub(quietLogger(), nil, limiter)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// A database write here would panic: rejected messages must never reach it.
		client := NewClient(hub, conn, &model.User{AutheliaSub: "limited"}, nil, hub.MessageService(nil), quietLogger())
		if !hub.RegisterClient(client) {
			conn.Close()
			return
		}
		go client.WritePump()
		client.ReadPump()
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]string{"text_content": "must not persist"}); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range bytes.Split(data, []byte{'\n'}) {
			var frame model.WSMessage
			if err := json.Unmarshal(line, &frame); err != nil {
				t.Fatal(err)
			}
			if frame.Type == "chat" {
				t.Fatal("rejected message broadcast")
			}
			if frame.Type == "error" {
				return
			}
		}
	}
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestBotConversationRoutingRevisionRestart(t *testing.T) {
	ctx := context.Background()
	groupA := "11111111-1111-4111-8111-111111111111"
	groupB := "22222222-2222-4222-8222-222222222222"
	at := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := []message{
		{ID: "a", SenderID: "human", Username: "human", SenderKind: "human", Text: "hello", CreatedAt: at, ChannelID: &groupA},
		{ID: "b", SenderID: "human", Username: "human", SenderKind: "human", Text: "hello", CreatedAt: at.Add(time.Second)},
	}
	revision := "1"
	missingRevision := false
	loseReply := true
	posted := map[string][]byte{}
	attempts := map[string]int{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if r.URL.Query().Get("scope") != "member" {
				t.Error("missing member scope")
			}
			if !missingRevision {
				w.Header().Set("X-Membership-Version", revision)
			}
			after, e := time.Parse(time.RFC3339Nano, r.URL.Query().Get("after"))
			if e != nil {
				t.Error(e)
			}
			page := []message{}
			for _, m := range rows {
				if m.CreatedAt.After(after) || (m.CreatedAt.Equal(after) && m.ID > r.URL.Query().Get("after_id")) {
					page = append(page, m)
				}
			}
			json.NewEncoder(w).Encode(page)
			return
		}
		key := r.Header.Get("Idempotency-Key")
		body, _ := io.ReadAll(r.Body)
		attempts[key]++
		if old, ok := posted[key]; ok && string(old) != string(body) {
			t.Error("pending retry body changed")
		}
		posted[key] = body
		if loseReply {
			loseReply = false
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(201)
		w.Write([]byte("{}"))
	}))
	defer server.Close()
	statePath := filepath.Join(t.TempDir(), "member.json")
	makeBot := func() *bot {
		b := &bot{client: server.Client(), syncURL: server.URL + "/sync", sendURL: server.URL + "/send", user: "bot", password: "synthetic", selfID: "self", statePath: statePath, now: time.Now, memberScope: true}
		if e := b.load(); e != nil {
			t.Fatal(e)
		}
		return b
	}
	b := makeBot()
	var retry *retryError
	if e := b.poll(ctx); !errors.As(e, &retry) {
		t.Fatalf("expected lost response: %v", e)
	}
	b = makeBot()
	if b.state.Pending == nil {
		t.Fatal("pending private reply lost")
	}
	if e := b.poll(ctx); e != nil {
		t.Fatal(e)
	}
	if attempts["pourover-reply-a"] != 2 || attempts["pourover-reply-b"] != 1 {
		t.Fatal("incorrect replay attempts")
	}
	for id, want := range map[string]*string{"a": &groupA, "b": nil} {
		var body struct {
			Channel *string `json:"channel_id"`
		}
		if e := json.Unmarshal(posted["pourover-reply-"+id], &body); e != nil {
			t.Fatal(e)
		}
		if (want == nil) != (body.Channel == nil) || (want != nil && *want != *body.Channel) {
			t.Fatal("reply changed conversation")
		}
	}
	// A newly joined group has older history than the durable merged checkpoint.
	rows = append([]message{{ID: "old", SenderID: "human", Username: "human", SenderKind: "human", CreatedAt: at.Add(-time.Hour), ChannelID: &groupB}}, rows...)
	revision = "2"
	b = makeBot()
	if e := b.poll(ctx); e != nil {
		t.Fatal(e)
	}
	if !b.state.Cursor.After.Equal(time.Unix(0, 0)) {
		t.Fatal("revision did not reset merged cursor")
	}
	b = makeBot() // restart between invalidation and backfill
	if e := b.poll(ctx); e != nil {
		t.Fatal(e)
	}
	if attempts["pourover-reply-old"] != 1 || attempts["pourover-reply-a"] != 2 || attempts["pourover-reply-b"] != 1 {
		t.Fatal("backfill duplicated old replies or missed new history")
	}
	if b.state.Revision != "2" || b.state.Conversations[groupB].ID != "old" {
		t.Fatal("revision or per-conversation state not retained")
	}
	before := b.state.Cursor
	missingRevision = true
	if e := b.poll(ctx); e == nil || b.state.Cursor != before {
		t.Fatal("missing revision advanced cursor")
	}
}

func TestBotPrivateDenialNeverFallsBack(t *testing.T) {
	group := "11111111-1111-4111-8111-111111111111"
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("X-Membership-Version", "1")
			json.NewEncoder(w).Encode([]message{{ID: "one", SenderID: "human", Username: "human", SenderKind: "human", CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ChannelID: &group}})
			return
		}
		posts++
		var body struct {
			Channel *string `json:"channel_id"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil || body.Channel == nil || *body.Channel != group {
			t.Error("private reply fell back to lounge")
		}
		w.WriteHeader(403)
	}))
	defer server.Close()
	b := &bot{client: server.Client(), syncURL: server.URL + "/sync", sendURL: server.URL + "/send", user: "bot", selfID: "self", statePath: filepath.Join(t.TempDir(), "state.json"), now: time.Now, memberScope: true}
	if e := b.load(); e != nil {
		t.Fatal(e)
	}
	if e := b.poll(context.Background()); e == nil {
		t.Fatal("403 not surfaced")
	}
	if posts != 1 || b.state.Pending == nil || !b.state.Cursor.After.Equal(time.Unix(0, 0)) {
		t.Fatal("denied operation lost or cursor advanced")
	}
	if e := b.load(); e != nil {
		t.Fatal(e)
	}
	if e := b.poll(context.Background()); e == nil || posts != 2 || b.state.Pending == nil {
		t.Fatal("restart did not retain private operation")
	}
}

package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestScopedSyncEqualTimePrivacyRevisionBackfill(t *testing.T) {
	db, c := managementDB(t)
	ctx := context.Background()
	r := NewMessageRepository(db)
	a := managementCreate(t, c, "a", nil, PrincipalRef{"human", "b"}, PrincipalRef{"bot", schemaBot})
	b := managementCreate(t, c, "outsider", nil, PrincipalRef{"human", "c"})
	abandoned := managementCreate(t, c, "a", nil, PrincipalRef{"bot", schemaBot})
	if err := c.ChangeMember(ctx, "a", *abandoned.ChannelID, PrincipalRef{"human", "a"}, false); err != nil {
		t.Fatal(err)
	}
	schemaExec(t, db, `INSERT INTO messages(id,sender_id,text_content,channel_id,created_at)
 SELECT ('10000000-0000-0000-0000-'||lpad((i*2)::text,12,'0'))::uuid,'a','readable',
 CASE WHEN i%2=0 THEN NULL ELSE $1::uuid END,'2020-01-01Z' FROM generate_series(1,1001) i`, *a.ChannelID)
	schemaExec(t, db, `INSERT INTO messages(id,sender_id,text_content,channel_id,created_at)
 SELECT ('10000000-0000-0000-0000-'||lpad((i*2+1)::text,12,'0'))::uuid,'outsider','private',
 CASE WHEN i%2=0 THEN $1::uuid ELSE $2::uuid END,'2020-01-01Z' FROM generate_series(1,1001) i`, *b.ChannelID, *abandoned.ChannelID)
	p := SendPrincipal{Kind: "human", ID: "b"}
	after := time.Unix(0, 0)
	afterID := ""
	seen := map[string]bool{}
	var version int64
	for n, want := range []int{500, 500, 1} {
		rows, v, err := r.GetScopedMessagesAfter(ctx, p, nil, true, after, afterID, 500)
		if err != nil || len(rows) != want {
			t.Fatalf("page %d count=%d err=%v", n, len(rows), err)
		}
		if n == 0 {
			version = v
		} else if version != v {
			t.Fatal("revision changed without mutation")
		}
		for _, m := range rows {
			if m.TextContent != "readable" || seen[m.ID] {
				t.Fatal("private/duplicate row")
			}
			seen[m.ID] = true
		}
		last := rows[len(rows)-1]
		after = last.CreatedAt
		afterID = last.ID
	}
	if len(seen) != 1001 {
		t.Fatalf("seen=%d", len(seen))
	}
	rows, v, err := r.GetScopedMessagesAfter(ctx, p, nil, true, after, afterID, 500)
	if err != nil || len(rows) != 0 || v != version {
		t.Fatalf("empty page revision: %d %v", v, err)
	}
	rows, _, err = r.GetScopedMessagesAfter(ctx, p, nil, false, time.Unix(0, 0), "", 500)
	if err != nil || len(rows) != 500 {
		t.Fatalf("legacy lounge: %v", err)
	}
	loungeIDs := map[string]bool{}
	for _, m := range rows {
		if m.ChannelID != nil {
			t.Fatal("legacy selector leaked group")
		}
		loungeIDs[m.ID] = true
	}
	recent, err := r.GetRecent(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range recent {
		if !loungeIDs[m.ID] || m.ChannelID != nil {
			t.Fatal("legacy history leaked group")
		}
	}
	if err = c.ChangeMember(ctx, "a", *a.ChannelID, PrincipalRef{"human", "b"}, false); err != nil {
		t.Fatal(err)
	}
	rows, v, err = r.GetScopedMessagesAfter(ctx, p, nil, true, time.Unix(0, 0), "", 500)
	if err != nil || v <= version {
		t.Fatalf("removal revision %d %v", v, err)
	}
	for _, m := range rows {
		if m.ChannelID != nil {
			t.Fatal("removed group returned")
		}
	}
	if _, _, err = r.GetScopedMessagesAfter(ctx, p, a.ChannelID, false, after, afterID, 500); !errors.Is(err, ErrConversationForbidden) {
		t.Fatalf("revoked direct=%v", err)
	}
	if err = c.ChangeMember(ctx, "a", *a.ChannelID, PrincipalRef{"human", "b"}, true); err != nil {
		t.Fatal(err)
	}
	_, joined, err := r.GetScopedMessagesAfter(ctx, p, nil, true, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), "", 500)
	if err != nil || joined <= v {
		t.Fatalf("empty-page addition revision %d %v", joined, err)
	}
	rows, _, err = r.GetScopedMessagesAfter(ctx, p, a.ChannelID, false, time.Unix(0, 0), "", 500)
	if err != nil || len(rows) != 500 {
		t.Fatalf("retained prejoin backfill %d %v", len(rows), err)
	}
	bot := SendPrincipal{Kind: "bot", ID: schemaBot}
	for _, channel := range []*string{b.ChannelID, abandoned.ChannelID} {
		if _, _, err = r.GetScopedMessagesAfter(ctx, bot, channel, false, after, afterID, 500); !errors.Is(err, ErrConversationForbidden) {
			t.Fatalf("bot denied=%v", err)
		}
	}
}

func TestScopedSendConcurrentReplayConflictAndRevocation(t *testing.T) {
	db, c := managementDB(t)
	r := NewMessageRepository(db)
	ctx := context.Background()
	a := managementCreate(t, c, "a", nil, PrincipalRef{"human", "b"})
	other := managementCreate(t, c, "a", nil, PrincipalRef{"human", "c"})
	p := SendPrincipal{Kind: "human", ID: "a", Username: "A"}
	const n = 24
	results := make(chan *SendResult, n)
	errs := make(chan error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			v, e := r.CreateIdempotentChannel(ctx, p, "same", "hello", "hello", a.ChannelID, 168*time.Hour, func() error { return nil })
			results <- v
			errs <- e
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var body []byte
	fresh := 0
	for v := range results {
		if !v.Replay {
			fresh++
		}
		if body == nil {
			body = v.Body
		} else if !bytes.Equal(body, v.Body) {
			t.Fatal("replay bytes changed")
		}
	}
	if fresh != 1 {
		t.Fatalf("fresh=%d", fresh)
	}
	var count int
	if e := db.QueryRow("SELECT count(*) FROM messages").Scan(&count); e != nil || count != 1 {
		t.Fatalf("rows=%d err=%v", count, e)
	}
	for _, tc := range []struct {
		text    string
		channel *string
	}{{"different", a.ChannelID}, {"hello", other.ChannelID}} {
		_, e := r.CreateIdempotentChannel(ctx, p, "same", tc.text, tc.text, tc.channel, 168*time.Hour, func() error { t.Fatal("conflict charged write"); return nil })
		if !errors.Is(e, ErrIdempotencyConflict) {
			t.Fatalf("conflict=%v", e)
		}
	}
	if e := c.ChangeMember(ctx, "b", *a.ChannelID, PrincipalRef{"human", "a"}, false); e != nil {
		t.Fatal(e)
	}
	for _, channel := range []*string{a.ChannelID, nil, other.ChannelID} {
		_, e := r.CreateIdempotentChannel(ctx, p, "same", "different", "different", channel, 168*time.Hour, func() error { t.Fatal("denied write"); return nil })
		if !errors.Is(e, ErrConversationForbidden) {
			t.Fatalf("authorization must precede replay/conflict: %v", e)
		}
	}
}

func TestScopedSendLegacyExactBytesAndLoungeFingerprint(t *testing.T) {
	db, _ := managementDB(t)
	r := NewMessageRepository(db)
	ctx := context.Background()
	p := SendPrincipal{Kind: "human", ID: "a", Username: "A"}
	sent, e := r.CreateIdempotent(ctx, p, "legacy", " old raw ", "old raw", 168*time.Hour, func() error { return nil })
	if e != nil {
		t.Fatal(e)
	}
	old := []byte(fmt.Sprintf(" {\"id\":%q,\"sender_id\":\"a\",\"sender_kind\":\"human\",\"username\":\"A\",\"text_content\":\"old raw\",\"created_at\":%q} \n", sent.Message.ID, sent.Message.CreatedAt.Format(time.RFC3339Nano)))
	schemaExec(t, db, "UPDATE message_idempotency SET response_body=$1 WHERE operation_key='legacy'", old)
	got, e := r.CreateIdempotentChannel(ctx, p, "legacy", " old raw ", "old raw", nil, 168*time.Hour, func() error { t.Fatal("replay charged write"); return nil })
	if e != nil || !got.Replay || !bytes.Equal(old, got.Body) || got.Message.ChannelID != nil {
		t.Fatalf("legacy bytes changed: %v", e)
	}
	var hash []byte
	if e = db.QueryRow("SELECT fingerprint FROM message_idempotency WHERE operation_key='legacy'").Scan(&hash); e != nil {
		t.Fatal(e)
	}
	want := sha256.Sum256([]byte(" old raw "))
	if !bytes.Equal(hash, want[:]) {
		t.Fatal("lounge hash incompatible")
	}
}

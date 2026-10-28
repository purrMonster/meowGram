package repository

import (
	"context"
	"fmt"
	"sort"
	"testing"
)

func TestConversationListTiesBoundedSummaryAndCursorBindings(t *testing.T) {
	db, r := managementDB(t)
	ctx := context.Background()
	refs := []PrincipalRef{{"human", "b"}}
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("member-%02d", i)
		schemaExec(t, db, "INSERT INTO users(authelia_sub,username) VALUES($1,$1)", id)
		refs = append(refs, PrincipalRef{"human", id})
	}
	ids := []string{}
	name := "named"
	for i := 0; i < 3; i++ {
		c := managementCreate(t, r, "a", &name, refs...)
		ids = append(ids, *c.ChannelID)
	}
	schemaExec(t, db, "UPDATE conversations SET created_at='2020-01-01Z'")
	sort.Strings(ids)
	first, err := r.List(ctx, "a", 1, "")
	if err != nil || len(first.Conversations) != 1 || first.Conversations[0].ChannelID != nil || first.NextCursor == nil {
		t.Fatalf("lounge first: %+v %v", first, err)
	}
	cursor := *first.NextCursor
	if _, err = r.List(ctx, "b", 1, cursor); err != ErrConversationInput {
		t.Fatalf("actor cursor accepted: %v", err)
	}
	for i, want := range ids {
		p, e := r.List(ctx, "a", 1, cursor)
		if e != nil || len(p.Conversations) != 1 {
			t.Fatalf("page: %+v %v", p, e)
		}
		c := p.Conversations[0]
		if c.ChannelID == nil || *c.ChannelID != want || c.MemberSummary.Count != 14 || len(c.MemberSummary.Members) != 10 {
			t.Fatalf("tie/summary wrong: %+v", c)
		}
		if i < 2 {
			if p.NextCursor == nil {
				t.Fatal("missing cursor")
			}
			cursor = *p.NextCursor
		} else if p.NextCursor != nil {
			t.Fatal("unexpected extra page")
		}
	}
	p, err := r.Principals(ctx, "a", "member", 1, "")
	if err != nil || p.NextCursor == nil {
		t.Fatalf("picker: %+v %v", p, err)
	}
	if _, err = r.Principals(ctx, "a", "other", 1, *p.NextCursor); err != ErrConversationInput {
		t.Fatalf("filter cursor accepted: %v", err)
	}
	m, err := r.Members(ctx, "a", ids[0], 1, "")
	if err != nil || m.NextCursor == nil {
		t.Fatalf("members: %+v %v", m, err)
	}
	if _, err = r.Members(ctx, "a", ids[1], 1, *m.NextCursor); err != ErrConversationInput {
		t.Fatalf("group cursor accepted: %v", err)
	}
}

func TestConversationTypedReuseAndEqualTimeRead(t *testing.T) {
	db, r := managementDB(t)
	ctx := context.Background()
	schemaExec(t, db, "INSERT INTO users(authelia_sub,username) VALUES($1,'same textual ID')", schemaBot)
	human := managementCreate(t, r, "a", nil, PrincipalRef{"human", schemaBot})
	bot := managementCreate(t, r, "a", nil, PrincipalRef{"bot", schemaBot})
	if *human.ChannelID == *bot.ChannelID {
		t.Fatal("typed sets conflated")
	}
	for _, tc := range []struct {
		ref PrincipalRef
		id  string
	}{{PrincipalRef{"human", schemaBot}, *human.ChannelID}, {PrincipalRef{"bot", schemaBot}, *bot.ChannelID}} {
		c, newly, e := r.Create(ctx, "a", nil, []PrincipalRef{tc.ref}, true)
		if e != nil || newly || *c.ChannelID != tc.id {
			t.Fatalf("typed reuse: %v", e)
		}
	}
	id := *human.ChannelID
	schemaExec(t, db, `INSERT INTO messages(id,sender_id,text_content,channel_id,created_at) VALUES
 ('10000000-0000-0000-0000-000000000001',$1,'first',$2,'2020-01-01Z'),
 ('10000000-0000-0000-0000-000000000002',$1,'second',$2,'2020-01-01Z'),
 ('10000000-0000-0000-0000-000000000003','a','self',$2,'2020-01-01Z')`, schemaBot, id)
	if e := r.MarkRead(ctx, "a", &id, "10000000-0000-0000-0000-000000000001"); e != nil {
		t.Fatal(e)
	}
	p, e := r.List(ctx, "a", 100, "")
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, c := range p.Conversations {
		if c.ChannelID != nil && *c.ChannelID == id {
			found = true
			if c.UnreadCount != 1 {
				t.Fatalf("equal-time unread=%d", c.UnreadCount)
			}
		}
	}
	if !found {
		t.Fatal("missing group")
	}
	// Another viewer's watermark is independent.
	p, e = r.List(ctx, schemaBot, 100, "")
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range p.Conversations {
		if c.ChannelID != nil && *c.ChannelID == id && c.UnreadCount != 1 {
			t.Fatalf("other viewer unread=%d", c.UnreadCount)
		}
	}
}

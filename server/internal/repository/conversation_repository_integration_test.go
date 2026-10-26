package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"

	"meowgram/server/internal/testutil"
)

func managementDB(t *testing.T) (*sql.DB, *ConversationRepository) {
	t.Helper()
	db, m, _ := testutil.Database(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	schemaExec(t, db, `INSERT INTO users(authelia_sub,username) VALUES ('a','Person A'),('b','Person B'),('c','Person C'),('outsider','Person outsider'); INSERT INTO bot_principals(id,username) VALUES ('20000000-0000-0000-0000-000000000001','Person bot')`)
	return db, NewConversationRepository(db)
}
func managementCreate(t *testing.T, r *ConversationRepository, actor string, name *string, refs ...PrincipalRef) Conversation {
	t.Helper()
	c, _, err := r.Create(context.Background(), actor, name, refs, name == nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.ChannelID == nil {
		t.Fatal("group has nil channel")
	}
	return c
}
func managementRevision(t *testing.T, db *sql.DB, kind, id string) int64 {
	t.Helper()
	q := "SELECT membership_version FROM users WHERE authelia_sub=$1"
	if kind == "bot" {
		q = "SELECT membership_version FROM bot_principals WHERE id=$1"
	}
	var n int64
	if err := db.QueryRow(q, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestConversationRepositoryConcurrentReuseAndNamedFresh(t *testing.T) {
	db, r := managementDB(t)
	ctx := context.Background()
	const count = 24
	type result struct {
		c     Conversation
		fresh bool
		err   error
	}
	results := make(chan result, count)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			actor := "a"
			refs := []PrincipalRef{{"human", "b"}, {"bot", schemaBot}}
			if i%2 == 1 {
				actor = "b"
				refs = []PrincipalRef{{"bot", schemaBot}, {"human", "a"}}
			}
			c, f, e := r.Create(ctx, actor, nil, refs, true)
			results <- result{c, f, e}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	id := ""
	created := 0
	for got := range results {
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.c.ChannelID == nil {
			t.Fatal("nil group")
		}
		if id == "" {
			id = *got.c.ChannelID
		}
		if *got.c.ChannelID != id {
			t.Fatal("concurrent start created different groups")
		}
		if got.fresh {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created=%d want 1", created)
	}
	var rows int
	if err := db.QueryRow("SELECT count(*) FROM conversations").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("rows=%d err=%v", rows, err)
	}
	name := "named"
	seen := map[string]bool{id: true}
	for i := 0; i < 3; i++ {
		c := managementCreate(t, r, "a", &name, PrincipalRef{"human", "b"}, PrincipalRef{"bot", schemaBot})
		if seen[*c.ChannelID] {
			t.Fatal("named group reused")
		}
		seen[*c.ChannelID] = true
	}
}

func TestConversationRepositoryRenameCurrentSetAndMembershipRevision(t *testing.T) {
	db, r := managementDB(t)
	ctx := context.Background()
	c := managementCreate(t, r, "a", nil, PrincipalRef{"human", "b"})
	id := *c.ChannelID
	name := "  named  "
	renamed, err := r.Rename(ctx, "b", id, &name)
	if err != nil || renamed.Name == nil || *renamed.Name != "named" {
		t.Fatalf("rename=%+v err=%v", renamed, err)
	}
	newer := managementCreate(t, r, "a", nil, PrincipalRef{"human", "b"})
	if *newer.ChannelID == id {
		t.Fatal("named group eligible for reuse")
	}
	blank := " \t "
	renamed, err = r.Rename(ctx, "b", id, &blank)
	if err != nil || renamed.Name != nil {
		t.Fatalf("clear err=%v", err)
	}
	reused, created, err := r.Create(ctx, "b", nil, []PrincipalRef{{"human", "a"}}, true)
	if err != nil || created || *reused.ChannelID != id {
		t.Fatalf("oldest cleared group not reused: %+v created=%v err=%v", reused, created, err)
	}
	before := managementRevision(t, db, "human", "c")
	if err = r.ChangeMember(ctx, "b", id, PrincipalRef{"human", "c"}, true); err != nil {
		t.Fatal(err)
	}
	after := managementRevision(t, db, "human", "c")
	if after <= before {
		t.Fatal("join did not bump revision")
	}
	if err = r.ChangeMember(ctx, "a", id, PrincipalRef{"human", "c"}, true); err != nil {
		t.Fatal(err)
	}
	if managementRevision(t, db, "human", "c") != after {
		t.Fatal("idempotent add changed revision")
	}
	current := managementCreate(t, r, "c", nil, PrincipalRef{"human", "a"}, PrincipalRef{"human", "b"})
	if *current.ChannelID != id {
		t.Fatal("reuse ignored current membership set")
	}
	if err = r.ChangeMember(ctx, "c", id, PrincipalRef{"human", "b"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Rename(ctx, "b", id, nil); !errors.Is(err, ErrConversationForbidden) {
		t.Fatalf("removed human can rename: %v", err)
	}
	removed := managementRevision(t, db, "human", "b")
	if err = r.ChangeMember(ctx, "c", id, PrincipalRef{"human", "b"}, false); err != nil {
		t.Fatal(err)
	}
	if managementRevision(t, db, "human", "b") != removed {
		t.Fatal("idempotent removal changed revision")
	}
	for _, bad := range []string{id, "99999999-9999-9999-9999-999999999999"} {
		if _, err = r.Members(ctx, "outsider", bad, 50, ""); !errors.Is(err, ErrConversationForbidden) {
			t.Fatalf("denied/unknown mismatch: %v", err)
		}
	}
}

func TestConversationRepositoryLastHumanAndDeletionRevisions(t *testing.T) {
	db, r := managementDB(t)
	ctx := context.Background()
	c := managementCreate(t, r, "a", nil, PrincipalRef{"bot", schemaBot})
	before := managementRevision(t, db, "bot", schemaBot)
	if err := r.ChangeMember(ctx, "a", *c.ChannelID, PrincipalRef{"human", "a"}, false); err != nil {
		t.Fatal(err)
	}
	if managementRevision(t, db, "bot", schemaBot) <= before {
		t.Fatal("last human removal did not invalidate bot")
	}
	if _, err := r.Members(ctx, "a", *c.ChannelID, 50, ""); !errors.Is(err, ErrConversationForbidden) {
		t.Fatalf("last human retains access: %v", err)
	}
	page, err := r.List(ctx, "a", 100, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Conversations {
		if item.ChannelID != nil && *item.ChannelID == *c.ChannelID {
			t.Fatal("no-human group in list")
		}
	}
	other := managementCreate(t, r, "b", nil, PrincipalRef{"bot", schemaBot})
	before = managementRevision(t, db, "bot", schemaBot)
	schemaExec(t, db, "DELETE FROM users WHERE authelia_sub='b'")
	if managementRevision(t, db, "bot", schemaBot) <= before {
		t.Fatal("user deletion cascade did not invalidate bot")
	}
	var humans int
	if err = db.QueryRow("SELECT count(*) FROM conversation_members WHERE conversation_id=$1 AND member_kind='human'", *other.ChannelID).Scan(&humans); err != nil || humans != 0 {
		t.Fatalf("cascade humans=%d err=%v", humans, err)
	}
}

func TestConversationRepositoryPaginationAndPicker(t *testing.T) {
	_, r := managementDB(t)
	ctx := context.Background()
	var group Conversation
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("Group %d", i)
		group = managementCreate(t, r, "a", &name, PrincipalRef{"human", "b"}, PrincipalRef{"human", "c"}, PrincipalRef{"bot", schemaBot})
	}
	seen := map[string]bool{}
	cursor := ""
	for pageNo := 0; pageNo < 10; pageNo++ {
		p, e := r.List(ctx, "a", 2, cursor)
		if e != nil {
			t.Fatal(e)
		}
		for _, c := range p.Conversations {
			key := "lounge"
			if c.ChannelID != nil {
				key = *c.ChannelID
			}
			if seen[key] {
				t.Fatal("duplicate conversation across pages")
			}
			seen[key] = true
		}
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("listed %d want lounge + 4", len(seen))
	}
	seen = map[string]bool{}
	cursor = ""
	for i := 0; i < 10; i++ {
		p, e := r.Members(ctx, "a", *group.ChannelID, 2, cursor)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range p.Members {
			key := m.Kind + ":" + m.ID
			if seen[key] {
				t.Fatal("duplicate member")
			}
			seen[key] = true
		}
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	if len(seen) != 4 {
		t.Fatalf("members=%d", len(seen))
	}
	seen = map[string]bool{}
	cursor = ""
	for i := 0; i < 10; i++ {
		p, e := r.Principals(ctx, "a", "Person", 2, cursor)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range p.Principals {
			key := m.Kind + ":" + m.ID
			if seen[key] {
				t.Fatal("duplicate picker principal")
			}
			seen[key] = true
		}
		if p.NextCursor == nil {
			break
		}
		cursor = *p.NextCursor
	}
	if !seen["bot:"+schemaBot] || !seen["human:b"] || !seen["human:c"] {
		t.Fatalf("picker missing typed principals: %v", seen)
	}
	if _, err := r.List(ctx, "a", 2, "not-a-cursor"); !errors.Is(err, ErrConversationInput) {
		t.Fatalf("bad cursor: %v", err)
	}
}

func TestConversationRepositoryUnreadWatermarkAndMute(t *testing.T) {
	db, r := managementDB(t)
	ctx := context.Background()
	c := managementCreate(t, r, "a", nil, PrincipalRef{"human", "b"})
	id := *c.ChannelID
	schemaExec(t, db, `INSERT INTO messages(id,sender_id,text_content,channel_id,created_at) VALUES
 ('10000000-0000-0000-0000-000000000001','b','one',$1,'2020-01-01Z'),
 ('10000000-0000-0000-0000-000000000002','b','two',$1,'2020-01-02Z'),
 ('10000000-0000-0000-0000-000000000003','a','self',$1,'2020-01-03Z'),
 ('10000000-0000-0000-0000-000000000004','b','lounge',NULL,'2020-01-04Z'),
 ('10000000-0000-0000-0000-000000000005','b','future',$1,'2099-01-01Z')`, id)
	get := func() Conversation {
		t.Helper()
		p, e := r.List(ctx, "a", 100, "")
		if e != nil {
			t.Fatal(e)
		}
		for _, item := range p.Conversations {
			if item.ChannelID != nil && *item.ChannelID == id {
				return item
			}
		}
		t.Fatal("group missing")
		return Conversation{}
	}
	initial := get()
	if initial.UnreadCount < 2 {
		t.Fatalf("unread=%d", initial.UnreadCount)
	}
	if err := r.SetMute(ctx, "a", &id, true); err != nil {
		t.Fatal(err)
	}
	muted := get()
	if !muted.Muted || muted.UnreadCount != initial.UnreadCount {
		t.Fatal("mute altered unread count")
	}
	if err := r.MarkRead(ctx, "a", &id, "10000000-0000-0000-0000-000000000002"); err != nil {
		t.Fatal(err)
	}
	after := get()
	if after.UnreadCount != initial.UnreadCount-2 {
		t.Fatalf("unread transition %d -> %d", initial.UnreadCount, after.UnreadCount)
	}
	if err := r.MarkRead(ctx, "a", &id, "10000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatal(err)
	}
	if get().UnreadCount != after.UnreadCount {
		t.Fatal("watermark moved backwards")
	}
	for _, mid := range []string{"10000000-0000-0000-0000-000000000004", "10000000-0000-0000-0000-000000000005"} {
		if err := r.MarkRead(ctx, "a", &id, mid); !errors.Is(err, ErrConversationInput) {
			t.Fatalf("cross-group/future read: %v", err)
		}
	}
	if err := r.SetMute(ctx, "a", nil, true); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkRead(ctx, "a", nil, "10000000-0000-0000-0000-000000000004"); err != nil {
		t.Fatal(err)
	}
	var mid string
	if err := db.QueryRow("SELECT read_message_id FROM conversation_user_state WHERE user_sub='a' AND channel_id=$1", id).Scan(&mid); err != nil || mid != "10000000-0000-0000-0000-000000000002" {
		t.Fatalf("watermark=%s err=%v", mid, err)
	}
}

func TestConversationRepositoryConcurrentMutationAndStart(t *testing.T) {
	for _, mode := range []string{"rename-clear", "membership"} {
		t.Run(mode, func(t *testing.T) {
			_, r := managementDB(t)
			ctx := context.Background()
			group := managementCreate(t, r, "a", nil, PrincipalRef{"human", "b"})
			id := *group.ChannelID
			start := make(chan struct{})
			errs := make(chan error, 24)
			var wg sync.WaitGroup
			for i := 0; i < 24; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					var err error
					if i%3 == 0 {
						if mode == "rename-clear" {
							var name *string
							if i%2 == 0 {
								value := "temporary"
								name = &value
							}
							_, err = r.Rename(ctx, "b", id, name)
						} else {
							err = r.ChangeMember(ctx, "b", id, PrincipalRef{"human", "c"}, i%2 == 0)
						}
					} else {
						refs := []PrincipalRef{{"human", "b"}}
						if mode == "membership" && i%2 == 0 {
							refs = append(refs, PrincipalRef{"human", "c"})
						}
						_, _, err = r.Create(ctx, "a", nil, refs, true)
					}
					errs <- err
				}(i)
			}
			close(start)
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := r.Rename(ctx, "a", id, nil); err != nil {
				t.Fatal(err)
			}
			if mode == "membership" {
				if err := r.ChangeMember(ctx, "a", id, PrincipalRef{"human", "c"}, true); err != nil {
					t.Fatal(err)
				}
			}
			page, err := r.Members(ctx, "a", id, 100, "")
			if err != nil {
				t.Fatal(err)
			}
			refs := []PrincipalRef{}
			for _, m := range page.Members {
				if m.Kind != "human" || m.ID != "a" {
					refs = append(refs, PrincipalRef{m.Kind, m.ID})
				}
			}
			first, created, err := r.Create(ctx, "a", nil, refs, true)
			if err != nil || created {
				t.Fatalf("eventual reuse created=%v err=%v", created, err)
			}
			for i := 0; i < 4; i++ {
				next, newly, err := r.Create(ctx, "a", nil, refs, true)
				if err != nil || newly || *next.ChannelID != *first.ChannelID {
					t.Fatalf("unstable eventual reuse new=%v err=%v", newly, err)
				}
			}
		})
	}
}

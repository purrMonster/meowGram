package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Every page uses one snapshot: authorization, membership summary and unread
// counts cannot be assembled from opposite sides of a membership change.
func (r *ConversationRepository) read(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

const principalRows = `SELECT 'human'::text AS kind,authelia_sub::text AS id,username::text AS label FROM users
 UNION ALL SELECT 'bot',id::text,username FROM bot_principals`

func conversationView(ctx context.Context, tx *sql.Tx, actor string, channel *string) (Conversation, error) {
	v := Conversation{ChannelID: channel, Kind: "lounge", MemberSummary: MemberSummary{Members: []ConversationMember{}}}
	if channel != nil {
		v.Kind = "group"
		err := tx.QueryRowContext(ctx, `SELECT c.name,coalesce((SELECT max(created_at) FROM messages WHERE channel_id=c.id),c.created_at)
   FROM conversations c WHERE c.id=$1 AND EXISTS(SELECT 1 FROM conversation_members m
    WHERE m.conversation_id=c.id AND m.member_kind='human' AND m.human_sub=$2)`, *channel, actor).Scan(&v.Name, &v.LastActivityAt)
		if errors.Is(err, sql.ErrNoRows) {
			return v, ErrConversationForbidden
		}
		if err != nil {
			return v, err
		}
	} else {
		if err := tx.QueryRowContext(ctx, "SELECT max(created_at) FROM messages WHERE channel_id IS NULL").Scan(&v.LastActivityAt); err != nil {
			return v, err
		}
	}
	if v.LastActivityAt != nil {
		at := v.LastActivityAt.UTC()
		v.LastActivityAt = &at
	}
	source := principalRows
	args := []any{}
	if channel != nil {
		source = `SELECT m.member_kind AS kind,coalesce(m.human_sub,m.bot_id::text) AS id,coalesce(u.username,b.username)::text AS label
   FROM conversation_members m LEFT JOIN users u ON m.member_kind='human' AND u.authelia_sub=m.human_sub
   LEFT JOIN bot_principals b ON m.member_kind='bot' AND b.id=m.bot_id WHERE m.conversation_id=$1`
		args = append(args, *channel)
	}
	rows, err := tx.QueryContext(ctx, `SELECT kind,id,label,count(*) OVER() FROM (`+source+`) p ORDER BY kind COLLATE "C",id COLLATE "C" LIMIT 10`, args...)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var p ConversationMember
		if err = rows.Scan(&p.Kind, &p.ID, &p.DisplayLabel, &v.MemberSummary.Count); err != nil {
			rows.Close()
			return v, err
		}
		v.MemberSummary.Members = append(v.MemberSummary.Members, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	var readAt *time.Time
	var readID *string
	err = tx.QueryRowContext(ctx, `SELECT muted,read_at,read_message_id FROM conversation_user_state
  WHERE user_sub=$1 AND channel_id IS NOT DISTINCT FROM $2::uuid`, actor, channel).Scan(&v.Muted, &readAt, &readID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return v, err
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM messages WHERE channel_id IS NOT DISTINCT FROM $1::uuid
  AND sender_id IS DISTINCT FROM $2 AND ($3::timestamptz IS NULL OR (created_at,id)>($3,$4::uuid))`, channel, actor, readAt, readID).Scan(&v.UnreadCount)
	return v, err
}

func (r *ConversationRepository) List(ctx context.Context, actor string, limit int, raw string) (ConversationPage, error) {
	page := ConversationPage{Conversations: []Conversation{}}
	limit, err := pageLimit(limit, 50)
	if err != nil {
		return page, err
	}
	cursor, err := decodeConversationCursor(raw, "list", actor, "", "")
	if err != nil {
		return page, err
	}
	err = r.read(ctx, func(tx *sql.Tx) error {
		remaining := limit
		if raw == "" {
			lounge, e := conversationView(ctx, tx, actor, nil)
			if e != nil {
				return e
			}
			page.Conversations = append(page.Conversations, lounge)
			remaining--
		}
		var at any
		var id any
		if raw != "" && !cursor.Start {
			at = cursor.At
			id = cursor.ID
		}
		rows, e := tx.QueryContext(ctx, `SELECT id,activity FROM (
   SELECT c.id,coalesce((SELECT max(created_at) FROM messages WHERE channel_id=c.id),c.created_at) AS activity
   FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id
   WHERE m.member_kind='human' AND m.human_sub=$1) visible
   WHERE $2::timestamptz IS NULL OR activity<$2 OR (activity=$2 AND id>$3::uuid)
   ORDER BY activity DESC,id LIMIT $4`, actor, at, id, remaining+1)
		if e != nil {
			return e
		}
		type item struct {
			id string
			at time.Time
		}
		items := []item{}
		for rows.Next() {
			var x item
			if e = rows.Scan(&x.id, &x.at); e != nil {
				rows.Close()
				return e
			}
			items = append(items, x)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		more := len(items) > remaining
		if more {
			items = items[:remaining]
		}
		for _, x := range items {
			v, e := conversationView(ctx, tx, actor, &x.id)
			if e != nil {
				return e
			}
			page.Conversations = append(page.Conversations, v)
		}
		if more {
			cursor.Start = len(items) == 0
			if !cursor.Start {
				x := items[len(items)-1]
				cursor.ID = x.id
				cursor.At = x.at.UTC()
			}
			page.NextCursor = encodeConversationCursor(cursor)
		}
		return nil
	})
	return page, err
}

func (r *ConversationRepository) Members(ctx context.Context, actor, id string, limit int, raw string) (MemberPage, error) {
	page := MemberPage{Members: []ConversationMember{}}
	id, err := ParseChannel(id)
	if err != nil {
		return page, err
	}
	limit, err = pageLimit(limit, 50)
	if err != nil {
		return page, err
	}
	cursor, err := decodeConversationCursor(raw, "members", actor, "", id)
	if err != nil {
		return page, err
	}
	err = r.read(ctx, func(tx *sql.Tx) error {
		refs, e := memberRefs(ctx, tx, id)
		if e != nil {
			return e
		}
		if e = requireHuman(refs, actor); e != nil {
			return e
		}
		rows, e := tx.QueryContext(ctx, `SELECT kind,id,label FROM (
   SELECT m.member_kind AS kind,coalesce(m.human_sub,m.bot_id::text) AS id,coalesce(u.username,b.username) AS label
   FROM conversation_members m LEFT JOIN users u ON m.member_kind='human' AND u.authelia_sub=m.human_sub
   LEFT JOIN bot_principals b ON m.member_kind='bot' AND b.id=m.bot_id WHERE m.conversation_id=$1) p
   WHERE (kind COLLATE "C",id COLLATE "C")>($2::text COLLATE "C",$3::text COLLATE "C")
   ORDER BY kind COLLATE "C",id COLLATE "C" LIMIT $4`, id, cursor.Kind, cursor.ID, limit+1)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var p ConversationMember
			if e = rows.Scan(&p.Kind, &p.ID, &p.DisplayLabel); e != nil {
				return e
			}
			page.Members = append(page.Members, p)
		}
		if e = rows.Err(); e != nil {
			return e
		}
		if len(page.Members) > limit {
			page.Members = page.Members[:limit]
			last := page.Members[limit-1]
			cursor.Kind = last.Kind
			cursor.ID = last.ID
			page.NextCursor = encodeConversationCursor(cursor)
		}
		return nil
	})
	return page, err
}
func (r *ConversationRepository) Principals(ctx context.Context, actor, q string, limit int, raw string) (PrincipalPage, error) {
	page := PrincipalPage{Principals: []ConversationMember{}}
	if !utf8.ValidString(q) || strings.ContainsRune(q, 0) || utf8.RuneCountInString(q) < 1 || utf8.RuneCountInString(q) > 100 {
		return page, ErrConversationInput
	}
	limit, err := pageLimit(limit, 20)
	if err != nil || limit > 50 {
		return page, ErrConversationInput
	}
	cursor, err := decodeConversationCursor(raw, "principals", actor, q, "")
	if err != nil {
		return page, err
	}
	err = r.read(ctx, func(tx *sql.Tx) error {
		rows, e := tx.QueryContext(ctx, `SELECT kind,id,label FROM (`+principalRows+`) p
   WHERE strpos(lower(label),lower($1))>0
   AND (kind COLLATE "C",id COLLATE "C")>($2::text COLLATE "C",$3::text COLLATE "C")
   ORDER BY kind COLLATE "C",id COLLATE "C" LIMIT $4`, q, cursor.Kind, cursor.ID, limit+1)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var p ConversationMember
			if e = rows.Scan(&p.Kind, &p.ID, &p.DisplayLabel); e != nil {
				return e
			}
			page.Principals = append(page.Principals, p)
		}
		if e = rows.Err(); e != nil {
			return e
		}
		if len(page.Principals) > limit {
			page.Principals = page.Principals[:limit]
			last := page.Principals[limit-1]
			cursor.Kind = last.Kind
			cursor.ID = last.ID
			page.NextCursor = encodeConversationCursor(cursor)
		}
		return nil
	})
	return page, err
}

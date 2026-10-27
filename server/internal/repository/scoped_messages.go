package repository

import (
	"context"
	"database/sql"
	"errors"
	"meowgram/server/internal/model"
	"time"
)

// Caller identity is supplied only by the verified OIDC/LDAP middleware.
func readableChannel(ctx context.Context, tx *sql.Tx, p SendPrincipal, channel *string, lock bool) error {
	if p.Kind != "human" && p.Kind != "bot" {
		return ErrConversationForbidden
	}
	if channel == nil {
		return nil
	}
	if _, err := ParseChannel(*channel); err != nil {
		return err
	}
	if lock {
		var id string
		err := tx.QueryRowContext(ctx, "SELECT id FROM conversations WHERE id=$1 FOR SHARE", *channel).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConversationForbidden
		}
		if err != nil {
			return err
		}
	}
	var allowed bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members m
  WHERE m.conversation_id=$1 AND m.member_kind=$2 AND coalesce(m.human_sub,m.bot_id::text)=$3)
  AND EXISTS(SELECT 1 FROM conversation_members h WHERE h.conversation_id=$1 AND h.member_kind='human')`, channel, p.Kind, p.ID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrConversationForbidden
	}
	return nil
}
func principalRevision(ctx context.Context, tx *sql.Tx, p SendPrincipal) (int64, error) {
	query := "SELECT membership_version FROM users WHERE authelia_sub=$1"
	if p.Kind == "bot" {
		query = "SELECT membership_version FROM bot_principals WHERE id=$1"
	} else if p.Kind != "human" {
		return 0, ErrConversationForbidden
	}
	var version int64
	err := tx.QueryRowContext(ctx, query, p.ID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrConversationForbidden
	}
	return version, err
}

const scopedMessageColumns = `m.id,coalesce(m.sender_id,m.bot_principal_id::text),coalesce(u.username,b.username),m.text_content,m.created_at,
 CASE WHEN m.bot_principal_id IS NULL THEN 'human' ELSE 'bot' END,m.channel_id`
const scopedMessageJoins = ` LEFT JOIN users u ON u.authelia_sub=m.sender_id LEFT JOIN bot_principals b ON b.id=m.bot_principal_id `

func scanScopedMessages(rows *sql.Rows) ([]*model.Message, error) {
	defer rows.Close()
	messages := []*model.Message{}
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.SenderID, &m.Username, &m.TextContent, &m.CreatedAt, &m.SenderKind, &m.ChannelID); err != nil {
			return nil, err
		}
		m.CreatedAt = m.CreatedAt.UTC()
		messages = append(messages, &m)
	}
	return messages, rows.Err()
}

// A revision and the corresponding authorized page come from one MVCC snapshot.
// Clients restart an epoch backfill when the revision changes; no channel list
// supplied by a caller or encoded cursor is ever trusted as authorization.
func (r *MessageRepository) GetScopedMessagesAfter(ctx context.Context, p SendPrincipal, channel *string, memberScope bool, after time.Time, afterID string, limit int) ([]*model.Message, int64, error) {
	if memberScope && channel != nil {
		return nil, 0, ErrConversationInput
	}
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	if err = readableChannel(ctx, tx, p, channel, false); err != nil {
		return nil, 0, err
	}
	var version int64
	if memberScope {
		version, err = principalRevision(ctx, tx, p)
		if err != nil {
			return nil, 0, err
		}
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+scopedMessageColumns+" FROM messages m"+scopedMessageJoins+`
 WHERE (($4::boolean=false AND m.channel_id IS NOT DISTINCT FROM $5::uuid)
 OR ($4=true AND (m.channel_id IS NULL OR (
  EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=m.channel_id AND cm.member_kind=$6 AND coalesce(cm.human_sub,cm.bot_id::text)=$7)
  AND EXISTS(SELECT 1 FROM conversation_members h WHERE h.conversation_id=m.channel_id AND h.member_kind='human')))))
 AND (m.created_at>$1 OR (m.created_at=$1 AND ($2::text='' OR m.id>NULLIF($2::text,'')::uuid)))
 ORDER BY m.created_at,m.id LIMIT $3`, after, afterID, limit, memberScope, channel, p.Kind, p.ID)
	if err != nil {
		return nil, 0, err
	}
	messages, err := scanScopedMessages(rows)
	if err != nil {
		return nil, 0, err
	}
	if err = tx.Commit(); err != nil {
		return nil, 0, err
	}
	return messages, version, nil
}

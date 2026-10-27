package repository

import (
	"context"
	"database/sql"
	"fmt"
	"meowgram/server/internal/model"
	"time"
)

// Frozen verbatim read queries and scan behavior from v2.0.0 commit
// be15cec6ceed2cb4a21b69cfe5e4548d2bfd818c, message_repository.go
// (blob df37beb7b991a07647a28187207d57bec5654754).
// Schema-7 preservation tests exercise schema 6 before upgrade; the evolving
// current repository requires channel_id and must not define this historical oracle.
type frozenR20MessageRepository struct{ db *sql.DB }

// GetRecent retrieves the most recent N messages in chronological order (oldest to newest).
func (r *frozenR20MessageRepository) GetRecent(ctx context.Context, limit int) ([]*model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Subquery fetches the latest N records, outer query orders them chronologically (ASC)
	query := `
		SELECT m.id, COALESCE(m.sender_id, m.bot_principal_id::text), COALESCE(u.username, b.username) AS username, m.text_content, m.created_at,
		       CASE WHEN m.bot_principal_id IS NULL THEN 'human' ELSE 'bot' END
		FROM (
			SELECT id, sender_id, bot_principal_id, text_content, created_at
			FROM messages
			ORDER BY created_at DESC, id DESC
			LIMIT $1
		) m
		LEFT JOIN users u ON m.sender_id = u.authelia_sub
		LEFT JOIN bot_principals b ON m.bot_principal_id = b.id
		ORDER BY m.created_at ASC, m.id ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent messages: %w", err)
	}
	defer rows.Close()

	var messages []*model.Message
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.Username, &msg.TextContent, &msg.CreatedAt, &msg.SenderKind); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return messages, nil
}

// GetMessagesAfter retrieves up to `limit` messages created strictly after `after` timestamp,
// ordered chronologically (oldest to newest) to power deterministic catch-up sync (UST-1.4.3).
// Capped at 500 messages to prevent payload bloat.
func (r *frozenR20MessageRepository) GetMessagesAfter(ctx context.Context, after time.Time, afterID string, limit int) ([]*model.Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}

	query := `
		SELECT m.id, COALESCE(m.sender_id, m.bot_principal_id::text), COALESCE(u.username, b.username) AS username, m.text_content, m.created_at,
		       CASE WHEN m.bot_principal_id IS NULL THEN 'human' ELSE 'bot' END
		FROM messages m
		LEFT JOIN users u ON m.sender_id = u.authelia_sub
		LEFT JOIN bot_principals b ON m.bot_principal_id = b.id
		WHERE m.created_at > $1
		   OR (m.created_at = $1 AND ($2::text = '' OR m.id > NULLIF($2::text, '')::uuid))
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT $3;
	`

	rows, err := r.db.QueryContext(ctx, query, after, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query catch-up messages: %w", err)
	}
	defer rows.Close()

	messages := make([]*model.Message, 0)
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.Username, &msg.TextContent, &msg.CreatedAt, &msg.SenderKind); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return messages, nil
}

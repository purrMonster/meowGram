package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"meowgram/server/internal/model"
)

// MessageRepository handles message persistence and history queries in PostgreSQL.
type MessageRepository struct {
	db *sql.DB
}

// NewMessageRepository instantiates a new MessageRepository.
func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create inserts a new chat message into PostgreSQL and returns the persisted record.
// Enforces the "Persistence Before Broadcast" mandate: messages must land on disk before fan-out.
func (r *MessageRepository) Create(ctx context.Context, senderID string, username string, textContent string) (*model.Message, error) {
	senderID = strings.TrimSpace(senderID)
	textContent = strings.TrimSpace(textContent)

	if senderID == "" {
		return nil, fmt.Errorf("sender_id cannot be empty")
	}
	if textContent == "" {
		return nil, fmt.Errorf("text_content cannot be empty")
	}

	query := `
		INSERT INTO messages (sender_id, text_content)
		VALUES ($1, $2)
		RETURNING id, sender_id, text_content, created_at;
	`

	var msg model.Message
	err := r.db.QueryRowContext(ctx, query, senderID, textContent).Scan(
		&msg.ID,
		&msg.SenderID,
		&msg.TextContent,
		&msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert message into PostgreSQL: %w", err)
	}

	msg.Username = username
	msg.SenderKind = "human"
	return &msg, nil
}

// CreateBot persists for an already authorized bot principal. Profile existence
// is not authorization; the future HTTP adapter must recheck directory access.
func (r *MessageRepository) CreateBot(ctx context.Context, principalID, text string) (*model.Message, error) {
	text = strings.TrimSpace(text)
	if principalID == "" || text == "" {
		return nil, fmt.Errorf("bot principal and text are required")
	}
	var msg model.Message
	err := r.db.QueryRowContext(ctx, `WITH inserted AS (
		INSERT INTO messages(bot_principal_id,text_content) VALUES($1,$2)
		RETURNING id,bot_principal_id,text_content,created_at)
		SELECT m.id,m.bot_principal_id,p.username,m.text_content,m.created_at
		FROM inserted m JOIN bot_principals p ON p.id=m.bot_principal_id`, principalID, text).Scan(
		&msg.ID, &msg.SenderID, &msg.Username, &msg.TextContent, &msg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to persist bot message: %w", err)
	}
	msg.SenderKind = "bot"
	return &msg, nil
}

// GetRecent retrieves the most recent N messages in chronological order (oldest to newest).
func (r *MessageRepository) GetRecent(ctx context.Context, limit int) ([]*model.Message, error) {
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
			WHERE channel_id IS NULL
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
func (r *MessageRepository) GetMessagesAfter(ctx context.Context, after time.Time, afterID string, limit int) ([]*model.Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}

	query := `
		SELECT m.id, COALESCE(m.sender_id, m.bot_principal_id::text), COALESCE(u.username, b.username) AS username, m.text_content, m.created_at,
		       CASE WHEN m.bot_principal_id IS NULL THEN 'human' ELSE 'bot' END
		FROM messages m
		LEFT JOIN users u ON m.sender_id = u.authelia_sub
		LEFT JOIN bot_principals b ON m.bot_principal_id = b.id
		WHERE m.channel_id IS NULL AND (m.created_at > $1
		   OR (m.created_at = $1 AND ($2::text = '' OR m.id > NULLIF($2::text, '')::uuid)))
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

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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
	return &msg, nil
}

// GetRecent retrieves the most recent N messages in chronological order (oldest to newest).
func (r *MessageRepository) GetRecent(ctx context.Context, limit int) ([]*model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Subquery fetches the latest N records, outer query orders them chronologically (ASC)
	query := `
		SELECT m.id, m.sender_id, COALESCE(u.username, m.sender_id) AS username, m.text_content, m.created_at
		FROM (
			SELECT id, sender_id, text_content, created_at
			FROM messages
			ORDER BY created_at DESC
			LIMIT $1
		) m
		LEFT JOIN users u ON m.sender_id = u.authelia_sub
		ORDER BY m.created_at ASC;
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent messages: %w", err)
	}
	defer rows.Close()

	var messages []*model.Message
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.Username, &msg.TextContent, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return messages, nil
}

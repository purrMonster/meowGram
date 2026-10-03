package model

import "time"

// Message represents a chat message persisted in PostgreSQL.
type Message struct {
	ID          string    `json:"id"`
	SenderID    string    `json:"sender_id"`
	Username    string    `json:"username"`
	TextContent string    `json:"text_content"`
	CreatedAt   time.Time `json:"created_at"`
}

// WSMessage represents the structured JSON payload transmitted over WebSocket.
type WSMessage struct {
	Type        string    `json:"type"` // "chat", "system", "welcome", "history"
	ID          string    `json:"id,omitempty"`
	SenderID    string    `json:"sender_id,omitempty"`
	Username    string    `json:"username,omitempty"`
	TextContent string    `json:"text_content"`
	CreatedAt   time.Time `json:"created_at"`
}

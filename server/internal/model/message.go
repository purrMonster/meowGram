package model

import "time"

// Message represents a chat message persisted in PostgreSQL.
type Message struct {
	ChannelID   *string   `json:"channel_id"`
	ID          string    `json:"id"`
	SenderID    string    `json:"sender_id"`
	SenderKind  string    `json:"sender_kind"`
	Username    string    `json:"username"`
	TextContent string    `json:"text_content"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserPresence represents a connected active user's presence state in the lounge.
type UserPresence struct {
	Username string `json:"username"`
	Sub      string `json:"sub"`
	IsOnline bool   `json:"is_online"`
}

// WSMessage represents the structured JSON payload transmitted over WebSocket.
type WSMessage struct {
	ChannelID   *string        `json:"channel_id"`
	Type        string         `json:"type"` // "chat", "system", "welcome", "history", "presence"
	ID          string         `json:"id,omitempty"`
	SenderID    string         `json:"sender_id,omitempty"`
	SenderKind  string         `json:"sender_kind,omitempty"`
	Username    string         `json:"username,omitempty"`
	TextContent string         `json:"text_content,omitempty"`
	Users       []UserPresence `json:"users,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

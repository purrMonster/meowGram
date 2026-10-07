package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"meowgram/server/internal/model"
	"meowgram/server/internal/repository"

	"github.com/gorilla/websocket"
)

const (
	// Maximum duration permitted to write a message to the socket peer.
	writeWait = 10 * time.Second

	// Maximum duration permitted to wait for the next pong message from the peer.
	pongWait = 60 * time.Second

	// Period for sending ping frames to peer; must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum permitted message size from client (512 KB).
	maxMessageSize = 512 * 1024

	// Client send channel buffer size to absorb traffic bursts without dropping frames.
	sendBufferSize = 256

	// MaxTextRunes caps a single chat message. It keeps frames, rows and push
	// payloads small; longer messages are rejected with an error frame.
	MaxTextRunes = 4000
)

// InboundPayload represents an incoming client message format.
type InboundPayload struct {
	TextContent string `json:"text_content"`
}

// Client mediates communication between an authenticated peer's WebSocket connection and the Hub.
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan *model.WSMessage
	user    *model.User
	msgRepo *repository.MessageRepository
	logger  *slog.Logger
}

// NewClient instantiates a new Client instance.
func NewClient(
	hub *Hub,
	conn *websocket.Conn,
	user *model.User,
	msgRepo *repository.MessageRepository,
	logger *slog.Logger,
) *Client {
	return &Client{
		hub:     hub,
		conn:    conn,
		send:    make(chan *model.WSMessage, sendBufferSize),
		user:    user,
		msgRepo: msgRepo,
		logger:  logger,
	}
}

// ReadPump continuously pumps messages from the WebSocket connection to the Hub.
//
// Concurrency Architecture:
// The application runs one ReadPump per client in its own dedicated goroutine.
// To satisfy the strict "Persistence Before Broadcast" mandate, incoming messages
// are synchronously committed to PostgreSQL before being dispatched to the Hub's Broadcast channel.
// If the database write fails, the message is dropped and NOT broadcasted to peers.
func (c *Client) ReadPump() {
	defer func() {
		// Signal Hub to unregister client and close connection upon reader termination.
		// UnregisterClient never blocks once the Hub has shut down.
		c.hub.UnregisterClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("WebSocket closed unexpectedly",
					"error", err,
					"sub", c.user.AutheliaSub,
					"username", c.user.Username,
				)
			} else {
				c.logger.Info("WebSocket closed normally",
					"sub", c.user.AutheliaSub,
					"username", c.user.Username,
				)
			}
			break
		}

		// Only process text frames for chat messaging
		if messageType != websocket.TextMessage {
			continue
		}

		rawText := strings.TrimSpace(string(payload))
		if rawText == "" {
			continue
		}

		// Parse input: accept either raw string or JSON envelope {"text_content": "..."}
		textContent := rawText
		var parsed InboundPayload
		if err := json.Unmarshal(payload, &parsed); err == nil && parsed.TextContent != "" {
			textContent = strings.TrimSpace(parsed.TextContent)
		}

		if textContent == "" {
			continue
		}

		if utf8.RuneCountInString(textContent) > MaxTextRunes {
			c.sendError("Message too long: the limit is 4000 characters.")
			continue
		}

		// =====================================================================
		// MANDATE: Message Persistence Before Broadcast
		// Commit message to PostgreSQL ledger before pushing to Hub channel.
		// =====================================================================
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		persistedMsg, err := c.msgRepo.Create(dbCtx, c.user.AutheliaSub, c.user.Username, textContent)
		cancel()

		if err != nil {
			c.logger.Error("Failed to persist message to PostgreSQL; aborting broadcast",
				"error", err,
				"sub", c.user.AutheliaSub,
			)

			// Inform sender of write failure (routed through the Hub, which owns c.send)
			c.sendError("Message delivery failed: could not persist to database.")
			continue
		}

		c.logger.Debug("Message persisted to PostgreSQL successfully",
			"message_id", persistedMsg.ID,
			"sub", c.user.AutheliaSub,
			"username", c.user.Username,
		)

		// Dispatch verified persisted message to the Broadcast Hub
		broadcastMsg := &model.WSMessage{
			Type:        "chat",
			ID:          persistedMsg.ID,
			SenderID:    persistedMsg.SenderID,
			Username:    c.user.Username,
			TextContent: persistedMsg.TextContent,
			CreatedAt:   persistedMsg.CreatedAt,
		}

		if !c.hub.Publish(broadcastMsg) {
			return // Hub stopped (server shutting down)
		}
	}
}

// sendError queues an error frame for this client only.
func (c *Client) sendError(text string) {
	c.hub.SendTo(c, &model.WSMessage{
		Type:        "error",
		TextContent: text,
		CreatedAt:   time.Now().UTC(),
	})
}

// WritePump continuously drains the client's send channel and pushes messages to the WebSocket.
//
// Concurrency Architecture:
// A dedicated WritePump goroutine ensures all writes to the underlying WebSocket connection
// are strictly serialized. Gorilla WebSocket connections do not permit concurrent writes.
// This goroutine also handles periodic heartbeat ping frames.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The Hub closed the channel during unregistration
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Encode message envelope to JSON format
			data, err := json.Marshal(message)
			if err != nil {
				c.logger.Error("Failed to marshal WSMessage to JSON", "error", err)
				continue
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(data)

			// Flush any additional queued messages into the same write buffer to optimize network I/O
			n := len(c.send)
			for i := 0; i < n; i++ {
				extraMsg := <-c.send
				extraData, err := json.Marshal(extraMsg)
				if err == nil {
					_, _ = w.Write([]byte{'\n'})
					_, _ = w.Write(extraData)
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			// Send periodic WebSocket ping frame to keep NAT/firewalls and browser state alive
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SendHistory loads recent persisted messages from PostgreSQL and queues them for
// this client via the Hub. Call it after the client has been registered.
func (c *Client) SendHistory(ctx context.Context, limit int) {
	messages, err := c.msgRepo.GetRecent(ctx, limit)
	if err != nil {
		c.logger.Warn("Failed to load message history for client", "error", err)
		return
	}

	for _, msg := range messages {
		envelope := &model.WSMessage{
			Type:        "history",
			ID:          msg.ID,
			SenderID:    msg.SenderID,
			Username:    msg.Username,
			TextContent: msg.TextContent,
			CreatedAt:   msg.CreatedAt,
		}

		if !c.hub.SendTo(c, envelope) {
			return
		}
	}
}

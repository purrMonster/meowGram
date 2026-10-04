package chat

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"meowgram/server/internal/model"
)

// Hub maintains the set of active connected WebSocket clients and serializes
// broadcasting operations across all subscriber channels.
//
// Concurrency Architecture:
// The Hub operates as a centralized single-threaded event loop driven by Go channels.
// By handling registrations, unregistrations, and message fan-outs sequentially in a single
// goroutine, access to the internal `clients` map is strictly thread-safe without requiring
// explicit mutex locks.
type Hub struct {
	// Registered active clients.
	clients map[*Client]bool

	// Inbound messages to be broadcasted to all connected clients.
	Broadcast chan *model.WSMessage

	// Register requests from newly upgraded WebSocket clients.
	Register chan *Client

	// Unregister requests from disconnecting or terminated clients.
	Unregister chan *Client

	logger *slog.Logger
}

// NewHub initializes and returns a new Hub instance.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		Broadcast:  make(chan *model.WSMessage, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		logger:     logger,
	}
}

// Run executes the central event coordination loop for the Hub.
// It must be launched in its own dedicated background goroutine.
func (h *Hub) Run(ctx context.Context) {
	h.logger.Info("Real-Time Broadcast Hub initialized and running")

	for {
		select {
		// 1. Client Registration
		case client := <-h.Register:
			h.clients[client] = true
			activeCount := len(h.clients)
			h.logger.Info("Client registered with Hub",
				"sub", client.user.AutheliaSub,
				"username", client.user.Username,
				"active_clients", activeCount,
			)

			// Notify lounge members of the new arrival
			h.broadcastSystemNotice(client.user.Username + " pounced into the lounge! 🐾")

			// Broadcast active presence roster to all connected clients
			h.broadcastPresence()

		// 2. Client Unregistration
		case client := <-h.Unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				activeCount := len(h.clients)

				h.logger.Info("Client unregistered from Hub",
					"sub", client.user.AutheliaSub,
					"username", client.user.Username,
					"active_clients", activeCount,
				)

				// Notify lounge members that the user departed
				h.broadcastSystemNotice(client.user.Username + " slinked away from the lounge. 💤")

				// Broadcast updated presence roster
				h.broadcastPresence()
			}

		// 3. Message Fan-Out / Broadcast
		case message := <-h.Broadcast:
			// Non-blocking fan-out across all active subscriber channels
			for client := range h.clients {
				select {
				case client.send <- message:
					// Message queued successfully in client's individual send buffer
				default:
					// Backpressure defense: If client's send buffer is saturated (slow consumer),
					// drop the connection to prevent blocking the entire broadcast hub.
					h.logger.Warn("Dropping slow or unresponsive client from Hub",
						"sub", client.user.AutheliaSub,
						"username", client.user.Username,
					)
					close(client.send)
					delete(h.clients, client)
				}
			}

		// 4. Graceful Shutdown
		case <-ctx.Done():
			h.logger.Info("Shutting down Broadcast Hub; evicting connected clients")
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			return
		}
	}
}

// ClientCount returns the current count of connected clients.
func (h *Hub) ClientCount() int {
	return len(h.clients)
}

func (h *Hub) broadcastSystemNotice(content string) {
	notice := &model.WSMessage{
		Type:        "system",
		Username:    "meowGram System",
		TextContent: content,
		CreatedAt:   time.Now().UTC(),
	}

	for client := range h.clients {
		select {
		case client.send <- notice:
		default:
		}
	}
}

func (h *Hub) broadcastPresence() {
	userMap := make(map[string]model.UserPresence)
	for client := range h.clients {
		if client.user != nil {
			sub := client.user.AutheliaSub
			username := client.user.Username
			if strings.TrimSpace(username) == "" {
				username = sub
			}
			userMap[sub] = model.UserPresence{
				Username: username,
				Sub:      sub,
				IsOnline: true,
			}
		}
	}

	users := make([]model.UserPresence, 0, len(userMap))
	for _, u := range userMap {
		users = append(users, u)
	}

	sort.Slice(users, func(i, j int) bool {
		return strings.ToLower(users[i].Username) < strings.ToLower(users[j].Username)
	})

	presenceMsg := &model.WSMessage{
		Type:      "presence",
		Users:     users,
		CreatedAt: time.Now().UTC(),
	}

	for client := range h.clients {
		select {
		case client.send <- presenceMsg:
		default:
		}
	}
}

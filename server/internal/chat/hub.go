package chat

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"meowgram/server/internal/model"
)

// PushPublisher sends a push notification to every device subscribed to a topic.
// It is satisfied by *fcm.Service; tests can supply a fake.
type PushPublisher interface {
	PublishToTopic(ctx context.Context, topic, title, body string, data map[string]string) error
}

const (
	// PushTopic is the FCM topic every signed-in client subscribes to.
	PushTopic = "room_lounge"

	// Push notifications are content-free (no message text, no sender) because FCM
	// topics have no access control: any app install can subscribe to them.
	pushTitle = "meowGram"
	pushBody  = "New messages in the lounge 🐾"

	// minPushInterval throttles lounge-activity pushes; notifications collapse on
	// the device anyway, so one per interval is enough.
	minPushInterval = 30 * time.Second
)

// directMessage is a frame addressed to a single client (history replay, error notices).
type directMessage struct {
	client  *Client
	message *model.WSMessage
}

// Hub maintains the set of active connected WebSocket clients and serializes
// broadcasting operations across all subscriber channels.
//
// Concurrency Architecture:
// The Hub is a single-goroutine event loop. Only Run touches the clients map and
// only Run closes a client's send channel. Every other goroutine reaches a client
// through the Hub (Broadcast, SendTo), so no goroutine can ever send on a channel
// that the Hub has already closed.
type Hub struct {
	// Registered active clients.
	clients map[*Client]bool

	// Inbound messages to be broadcasted to all connected clients.
	Broadcast chan *model.WSMessage

	// Register requests from newly upgraded WebSocket clients.
	Register chan *Client

	// Unregister requests from disconnecting or terminated clients.
	Unregister chan *Client

	direct chan directMessage
	done   chan struct{}
	push   chan struct{}

	publisher PushPublisher
	logger    *slog.Logger
}

// NewHub initializes and returns a new Hub instance. publisher may be nil, in
// which case push notifications are disabled.
func NewHub(logger *slog.Logger, publisher PushPublisher) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		Broadcast:  make(chan *model.WSMessage, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		direct:     make(chan directMessage, 256),
		done:       make(chan struct{}),
		push:       make(chan struct{}, 1),
		publisher:  publisher,
		logger:     logger,
	}
}

// stopped reports whether Run has returned. Checked first because the buffered
// channels would otherwise accept (and silently strand) frames after shutdown.
func (h *Hub) stopped() bool {
	select {
	case <-h.done:
		return true
	default:
		return false
	}
}

// Done is closed when Run has returned.
func (h *Hub) Done() <-chan struct{} { return h.done }

// RegisterClient hands a client to the Hub. It returns false if the Hub has stopped.
func (h *Hub) RegisterClient(c *Client) bool {
	select {
	case h.Register <- c:
		return true
	case <-h.done:
		return false
	}
}

// UnregisterClient removes a client from the Hub. It never blocks after shutdown.
func (h *Hub) UnregisterClient(c *Client) {
	select {
	case h.Unregister <- c:
	case <-h.done:
	}
}

// Publish queues a persisted message for fan-out. It returns false if the Hub has stopped.
func (h *Hub) Publish(msg *model.WSMessage) bool {
	if h.stopped() {
		return false
	}
	select {
	case h.Broadcast <- msg:
		return true
	case <-h.done:
		return false
	}
}

// SendTo queues a frame for a single client. The frame is dropped if the client is
// no longer registered or its buffer is full. It returns false if the Hub has stopped.
func (h *Hub) SendTo(c *Client, msg *model.WSMessage) bool {
	if h.stopped() {
		return false
	}
	select {
	case h.direct <- directMessage{client: c, message: msg}:
		return true
	case <-h.done:
		return false
	}
}

// Run executes the central event coordination loop for the Hub.
// It must be launched in its own dedicated background goroutine.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	h.logger.Info("Real-Time Broadcast Hub initialized and running")

	if h.publisher != nil {
		go h.pushWorker(ctx)
	}

	for {
		select {
		// 1. Client Registration
		case client := <-h.Register:
			h.clients[client] = true
			h.logger.Info("Client registered with Hub",
				"sub", client.user.AutheliaSub,
				"username", client.user.Username,
				"active_clients", len(h.clients),
			)
			h.broadcastSystemNotice(client.user.Username + " pounced into the lounge! 🐾")
			h.broadcastPresence()

		// 2. Client Unregistration
		case client := <-h.Unregister:
			if _, ok := h.clients[client]; ok {
				h.remove(client)
				h.logger.Info("Client unregistered from Hub",
					"sub", client.user.AutheliaSub,
					"username", client.user.Username,
					"active_clients", len(h.clients),
				)
				h.broadcastSystemNotice(client.user.Username + " slinked away from the lounge. 💤")
				h.broadcastPresence()
			}

		// 3. Single-client frames (history replay, error notices)
		case d := <-h.direct:
			if _, ok := h.clients[d.client]; ok {
				h.deliver(d.client, d.message)
			}

		// 4. Message Fan-Out / Broadcast
		case message := <-h.Broadcast:
			for client := range h.clients {
				h.deliver(client, message)
			}
			if message.Type == "chat" && h.publisher != nil {
				// Coalesce: at most one pending push request.
				select {
				case h.push <- struct{}{}:
				default:
				}
			}

		// 5. Graceful Shutdown
		case <-ctx.Done():
			h.logger.Info("Shutting down Broadcast Hub; evicting connected clients")
			for client := range h.clients {
				h.remove(client)
			}
			return
		}
	}
}

// deliver performs a non-blocking send; a client whose buffer is full is evicted
// so that a slow consumer can never stall the Hub (backpressure by eviction).
func (h *Hub) deliver(client *Client, message *model.WSMessage) {
	select {
	case client.send <- message:
	default:
		h.logger.Warn("Dropping slow or unresponsive client from Hub",
			"sub", client.user.AutheliaSub,
			"username", client.user.Username,
		)
		h.remove(client)
	}
}

// remove deletes a client and closes its send channel. Only called from Run.
func (h *Hub) remove(client *Client) {
	delete(h.clients, client)
	close(client.send)
}

// pushWorker sends at most one content-free lounge-activity push per minPushInterval.
func (h *Hub) pushWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.push:
		}

		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := h.publisher.PublishToTopic(sendCtx, PushTopic, pushTitle, pushBody,
			map[string]string{"type": "lounge_activity"})
		cancel()
		if err != nil {
			h.logger.Error("Failed to push FCM notification", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(minPushInterval):
		}
	}
}

func (h *Hub) broadcastSystemNotice(content string) {
	notice := &model.WSMessage{
		Type:        "system",
		Username:    "meowGram System",
		TextContent: content,
		CreatedAt:   time.Now().UTC(),
	}
	for client := range h.clients {
		h.deliver(client, notice)
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
		h.deliver(client, presenceMsg)
	}
}

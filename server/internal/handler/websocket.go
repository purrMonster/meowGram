package handler

import (
	"log/slog"
	"net/http"

	"meowgram/server/internal/auth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/config"
	"meowgram/server/internal/repository"

	"github.com/gorilla/websocket"
)

// WebSocketHandler manages protocol upgrade, client registration with the Hub,
// and launches dedicated ReadPump and WritePump goroutines.
func WebSocketHandler(
	hub *chat.Hub,
	msgRepo *repository.MessageRepository,
	cfg *config.Config,
	logger *slog.Logger,
) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			allowed := cfg.IsAllowedOrigin(origin)
			if !allowed {
				logger.Warn("WebSocket handshake rejected by CORS origin contract",
					"origin", origin,
					"remote_addr", r.RemoteAddr,
				)
			}
			return allowed
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Validate authenticated context from OIDC middleware
		user, hasUser := auth.UserFromContext(r.Context())
		if !hasUser || user == nil {
			http.Error(w, "Unauthorized: valid Authelia OIDC token required", http.StatusUnauthorized)
			return
		}

		// 2. Perform HTTP to WebSocket protocol upgrade
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Failed to upgrade WebSocket connection",
				"error", err,
				"remote_addr", r.RemoteAddr,
				"sub", user.AutheliaSub,
			)
			return
		}

		logger.Info("WebSocket peer upgraded and verified",
			"remote_addr", r.RemoteAddr,
			"sub", user.AutheliaSub,
			"username", user.Username,
		)

		// 3. Instantiate Client and register with the Broadcast Hub
		client := chat.NewClient(hub, conn, user, msgRepo, logger)
		hub.Register <- client

		// 4. Stream recent message history to this newly connected client
		client.SendHistory(r.Context(), 30)

		// 5. Concurrency Model: Spawn isolated read and write pumps
		// WritePump serializes all outgoing messages and ping heartbeats
		go client.WritePump()

		// ReadPump executes in its own goroutine, listening for client input and persisting to PostgreSQL
		go client.ReadPump()
	}
}

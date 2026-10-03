package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"meowgram/server/internal/auth"
	"meowgram/server/internal/config"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512 KB
)

// EchoWebSocketHandler handles incoming WebSocket connections, greeting authenticated users and echoing messages.
func EchoWebSocketHandler(cfg *config.Config, logger *slog.Logger) http.HandlerFunc {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			allowed := cfg.IsAllowedOrigin(origin)
			if !allowed {
				logger.Warn("WebSocket handshake rejected due to CORS origin policy",
					"origin", origin,
					"remote_addr", r.RemoteAddr,
				)
			}
			return allowed
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Extract authenticated user and sub claim from request context
		user, hasUser := auth.UserFromContext(r.Context())
		sub, _ := auth.SubFromContext(r.Context())

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("Failed to upgrade WebSocket connection", "error", err, "remote_addr", r.RemoteAddr)
			return
		}
		defer conn.Close()

		username := "Anonymous"
		userID := ""
		if hasUser && user != nil {
			username = user.Username
			userID = user.ID
		}

		logger.Info("WebSocket client connected",
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
			"user_id", userID,
			"username", username,
			"sub", sub,
		)

		conn.SetReadLimit(maxMessageSize)
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		// Send initial welcome frame acknowledging OIDC authenticated identity
		welcomeMsg := fmt.Sprintf("Welcome to meowGram Lounge, @%s! (ID: %s)", username, userID)
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(welcomeMsg))

		// Ping ticker goroutine
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		done := make(chan struct{})

		// Background ping writer
		go func() {
			for {
				select {
				case <-ticker.C:
					_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}
				case <-done:
					return
				}
			}
		}()

		// Echo loop
		for {
			messageType, payload, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					logger.Warn("WebSocket closed unexpectedly",
						"error", err,
						"remote_addr", r.RemoteAddr,
						"username", username,
					)
				} else {
					logger.Info("WebSocket closed normally",
						"remote_addr", r.RemoteAddr,
						"username", username,
					)
				}
				close(done)
				break
			}

			logger.Debug("WebSocket message received",
				"remote_addr", r.RemoteAddr,
				"username", username,
				"bytes", len(payload),
				"type", messageType,
			)

			// Formulate echo response tagged with the verified user's identity
			var responsePayload []byte
			if messageType == websocket.TextMessage {
				responsePayload = []byte(fmt.Sprintf("[%s]: %s", username, string(payload)))
			} else {
				responsePayload = payload
			}

			// Echo payload back to client
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(messageType, responsePayload); err != nil {
				logger.Error("Failed to echo WebSocket message",
					"error", err,
					"remote_addr", r.RemoteAddr,
					"username", username,
				)
				close(done)
				break
			}
		}
	}
}

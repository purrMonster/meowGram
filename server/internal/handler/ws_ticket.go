package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"meowgram/server/internal/repository"
	"net/http"

	"meowgram/server/internal/auth"
)

func WSTicketHandler(store *auth.WSTicketStore, logger *slog.Logger, conversations ...ConversationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		user, ok := auth.UserFromContext(r.Context())
		if !ok || user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// This validates the selector/access only. Ticket/channel binding and scoped
		// WebSocket consumption are introduced by UST-2.1.2.1.
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
		if err != nil {
			conversationError(w, repository.ErrConversationInput)
			return
		}
		var input struct {
			Channel *string `json:"channel_id"`
		}
		if len(bytes.TrimSpace(data)) > 0 {
			r.Body = io.NopCloser(bytes.NewReader(data))
			if _, err = conversationBody(w, r, &input); err != nil {
				conversationError(w, repository.ErrConversationInput)
				return
			}
		}
		if input.Channel != nil {
			channel, err := repository.ParseChannel(*input.Channel)
			if err != nil {
				conversationError(w, err)
				return
			}
			if len(conversations) == 0 || conversations[0] == nil {
				conversationError(w, repository.ErrConversationBusy)
				return
			}
			if _, err = conversations[0].Members(r.Context(), user.AutheliaSub, channel, 1, ""); err != nil {
				conversationError(w, err)
				return
			}
		}
		ticket, err := store.Issue(user)
		if err != nil {
			logger.Error("Failed to issue WebSocket ticket", "error", err)
			http.Error(w, "Unable to establish WebSocket session", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ticket":     ticket,
			"expires_in": 30,
		})
	}
}

// NoStore wraps authentication too, so rejected ticket requests are not cached.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

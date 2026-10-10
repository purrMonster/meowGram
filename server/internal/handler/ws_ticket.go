package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"meowgram/server/internal/auth"
)

func WSTicketHandler(store *auth.WSTicketStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		if !ok || user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
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

package handler

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"io"
	"math"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/repository"
	"net/http"
	"strconv"
)

func SendHandler(service *chat.MessageService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		kind, id, name := "", "", ""
		if p, ok := botauth.PrincipalFromContext(r.Context()); ok {
			kind, id, name = "bot", p.ID, p.DisplayName
		} else if u, ok := auth.UserFromContext(r.Context()); ok {
			kind, id, name = "human", u.AutheliaSub, u.Username
		}
		if id == "" {
			http.Error(w, "authentication required", 401)
			return
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			http.Error(w, "one idempotency key required", 400)
			return
		}
		key, err := uuid.Parse(keys[0])
		if err != nil || key == uuid.Nil {
			http.Error(w, "invalid idempotency key", 400)
			return
		}
		var body struct {
			Text string `json:"text_content"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF {
			http.Error(w, "invalid message body", 400)
			return
		}
		msg, replay, err := service.SendOnce(r.Context(), kind, id, name, key.String(), body.Text)
		if err != nil && !errors.Is(err, chat.ErrBroadcastStopped) {
			status := 503
			var limited *chat.MessageRateError
			switch {
			case errors.Is(err, repository.ErrReceiptConflict):
				status = 409
			case errors.Is(err, chat.ErrEmptyMessage), errors.Is(err, chat.ErrMessageTooLong):
				status = 400
			case errors.As(err, &limited):
				status = 429
				w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(limited.RetryAfter.Seconds())))))
			}
			http.Error(w, http.StatusText(status), status)
			return
		}
		status := 201
		if replay {
			status = 200
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": msg.ID, "idempotency_key": key.String(), "persisted": true})
	})
}

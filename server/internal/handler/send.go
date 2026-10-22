package handler

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/repository"
	"mime"
	"net/http"
	"strconv"
	"time"
)

func SendHandler(service *chat.MessageService, retention time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(405)
			return
		}
		var p repository.SendPrincipal
		if bot, ok := botauth.PrincipalFromContext(r.Context()); ok {
			p = repository.SendPrincipal{Kind: "bot", ID: bot.ID, Username: bot.Username}
		} else if user, ok := auth.UserFromContext(r.Context()); ok && user != nil {
			p = repository.SendPrincipal{Kind: "human", ID: user.AutheliaSub, Username: user.Username}
		} else {
			writeSyncJSONError(w, 401, "unauthorized", "authentication required")
			return
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 || len(keys[0]) < 1 || len(keys[0]) > 255 {
			writeSyncJSONError(w, 400, "invalid_key", "Idempotency-Key required (1-255 printable ASCII characters)")
			return
		}
		for _, c := range []byte(keys[0]) {
			if c < 32 || c > 126 {
				writeSyncJSONError(w, 400, "invalid_key", "invalid Idempotency-Key")
				return
			}
		}
		typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || typ != "application/json" {
			writeSyncJSONError(w, 415, "unsupported_media_type", "application/json required")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32768)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input struct {
			Text *string `json:"text_content"`
		}
		if err := decoder.Decode(&input); err != nil {
			var large *http.MaxBytesError
			status := 400
			if errors.As(err, &large) {
				status = 413
			}
			writeSyncJSONError(w, status, "invalid_body", "one text_content string required")
			return
		}
		if input.Text == nil {
			writeSyncJSONError(w, 400, "invalid_body", "text_content required")
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			writeSyncJSONError(w, 400, "invalid_body", "one JSON object required")
			return
		}
		result, err := service.SendIdempotent(r.Context(), p, keys[0], *input.Text, retention)
		if err != nil && !chat.IsDurableSend(err) {
			status, code := 503, "unavailable"
			var rate *chat.MessageRateError
			switch {
			case errors.Is(err, repository.ErrIdempotencyConflict):
				status, code = 409, "idempotency_conflict"
			case errors.Is(err, chat.ErrEmptyMessage), errors.Is(err, chat.ErrMessageTooLong):
				status, code = 400, "invalid_text"
			case errors.As(err, &rate):
				status, code = 429, "rate_limited"
				w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(rate.RetryAfter.Seconds())))))
			}
			if status == 503 {
				w.Header().Set("Retry-After", "1")
			}
			writeSyncJSONError(w, status, code, http.StatusText(status))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(result.Body)
	})
}

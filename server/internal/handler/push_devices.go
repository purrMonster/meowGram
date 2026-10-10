package handler

import (
	"context"
	"encoding/json"
	"io"
	"meowgram/server/internal/auth"
	"net/http"
	"strings"
	"time"
)

type DeviceRegistry interface {
	Register(context.Context, string, string, time.Time) error
	Unregister(context.Context, string, string) error
}

// Device identity always comes from verified bearer claims, never request JSON.
func PushDeviceHandler(registry DeviceRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		if !ok || user == nil {
			http.Error(w, "Unauthorized", 401)
			return
		}
		if r.Method != http.MethodPut && r.Method != http.MethodDelete {
			w.Header().Set("Allow", "PUT, DELETE")
			http.Error(w, "Method not allowed", 405)
			return
		}
		var input struct {
			Token string `json:"token"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			http.Error(w, "Invalid device registration", 400)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid device registration", 400)
			return
		}
		if input.Token == "" || len(input.Token) > 4096 || strings.ContainsAny(input.Token, " \t\r\n") {
			http.Error(w, "Invalid device token", 400)
			return
		}
		var err error
		if r.Method == http.MethodPut {
			err = registry.Register(r.Context(), user.AutheliaSub, input.Token, time.Now().Add(24*time.Hour))
		} else {
			err = registry.Unregister(r.Context(), user.AutheliaSub, input.Token)
		}
		if err != nil {
			http.Error(w, "Device registration unavailable", 503)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}
}

package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"meowgram/server/internal/auth"
	"meowgram/server/internal/model"
)

func TestTicketEndpointRequiresIdentityAndDisablesCaching(t *testing.T) {
	store := auth.NewWSTicketStore()
	handler := WSTicketHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest("POST", "/api/ws-ticket", nil)
	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, req)
	if unauth.Code != 401 {
		t.Fatalf("unauthenticated status %d", unauth.Code)
	}
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &model.User{ID: "user"}))
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != 200 || res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("ticket response is not successful and non-cacheable")
	}
	var body struct {
		Ticket    string `json:"ticket"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ExpiresIn != 30 {
		t.Fatal("unexpected expiry")
	}
	user, ok := store.Consume(body.Ticket)
	if !ok || user.ID != "user" {
		t.Fatal("ticket does not authenticate its user")
	}
	if _, ok = store.Consume(body.Ticket); ok {
		t.Fatal("ticket replay accepted")
	}
}

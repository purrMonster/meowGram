package middleware

import (
	"context"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/model"
	"meowgram/server/internal/ratelimit"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticatedRateBudgetAndRetryHeader(t *testing.T) {
	h := UserRateLimit(ratelimit.New(1, 1))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, tc := range []struct {
		sub  string
		want int
	}{{"", 401}, {"alice", 204}, {"alice", 429}, {"bob", 204}} {
		req := httptest.NewRequest("GET", "/api/messages/sync", nil)
		if tc.sub != "" {
			req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &model.User{AutheliaSub: tc.sub}))
		}
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != tc.want {
			t.Fatalf("status %d want %d", res.Code, tc.want)
		}
		if tc.want == 429 && res.Header().Get("Retry-After") == "" {
			t.Fatal("missing retry header")
		}
	}
}
func TestGlobalBudgetKeepsHealthAvailable(t *testing.T) {
	h := GlobalRateLimit(ratelimit.New(1, 1), "/ready")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, tc := range []struct {
		path string
		want int
	}{{"/ws", 204}, {"/api/ws-ticket", 429}, {"/ready", 204}, {"/healthz", 204}} {
		req := httptest.NewRequest("GET", tc.path, nil)
		req.Header.Set("X-Forwarded-For", "new-forged-address")
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != tc.want {
			t.Fatalf("status %d want %d", res.Code, tc.want)
		}
	}
}

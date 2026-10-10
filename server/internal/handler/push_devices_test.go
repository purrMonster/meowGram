package handler

import (
	"context"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/model"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeRegistry struct {
	sub, token string
	deleted    bool
	expiry     time.Time
}

func (f *fakeRegistry) Register(_ context.Context, sub, token string, expiry time.Time) error {
	f.sub = sub
	f.token = token
	f.expiry = expiry
	return nil
}
func (f *fakeRegistry) Unregister(_ context.Context, sub, token string) error {
	f.sub = sub
	f.token = token
	f.deleted = true
	return nil
}
func TestPushDeviceAuthorizationAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name, method, body string
		auth               bool
		code               int
	}{
		{"anonymous", "PUT", `{"token":"device"}`, false, 401},
		{"register", "PUT", `{"token":"device"}`, true, 204},
		{"remove", "DELETE", `{"token":"device"}`, true, 204},
		{"spoof owner", "PUT", `{"token":"device","user_sub":"someone-else"}`, true, 400},
		{"empty token", "PUT", `{"token":""}`, true, 400},
		{"extra JSON", "PUT", `{"token":"device"} {}`, true, 400},
		{"oversized", "PUT", `{"token":"` + strings.Repeat("a", 9000) + `"}`, true, 400},
		{"wrong method", "GET", `{"token":"device"}`, true, 405},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := &fakeRegistry{}
			req := httptest.NewRequest(tc.method, "/api/push/devices", strings.NewReader(tc.body))
			if tc.auth {
				req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &model.User{AutheliaSub: "verified-user"}))
			}
			res := httptest.NewRecorder()
			PushDeviceHandler(registry).ServeHTTP(res, req)
			if res.Code != tc.code {
				t.Fatalf("status %d, want %d", res.Code, tc.code)
			}
			if tc.code == 204 && registry.sub != "verified-user" {
				t.Fatal("unverified ownership")
			}
			if tc.code == 204 && tc.method == "PUT" && time.Until(registry.expiry) < 23*time.Hour {
				t.Fatal("missing lease")
			}
			if tc.code != 204 && registry.token != "" {
				t.Fatal("invalid request mutated registry")
			}
		})
	}
}

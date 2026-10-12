package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"meowgram/server/internal/config"
)

func TestClientConfigExposesOnlyPublicSignInSettings(t *testing.T) {
	cfg := &config.Config{
		AutheliaIssuer:     "https://authelia.example.home.arpa",
		AutheliaClientID:   "meowgram",
		WSEndpoint:         "/ws",
		WSTicketEndpoint:   "/api/ws-ticket",
		SyncEndpoint:       "/api/messages/sync",
		PushDeviceEndpoint: "/api/push/devices",
		HealthEndpoint:     "/healthz",
		DatabaseURL:        "postgres://meowgram:secret@db/meowgram",
		GoogleCredentials:  "/run/secrets/fcm.json",
	}
	res := httptest.NewRecorder()
	ClientConfigHandler(cfg).ServeHTTP(res, httptest.NewRequest("GET", ClientConfigPath, nil))

	if res.Code != 200 {
		t.Fatalf("status %d", res.Code)
	}
	if ct := res.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content type %q", ct)
	}
	var got map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"issuer":            "https://authelia.example.home.arpa",
		"client_id":         "meowgram",
		"ws_path":           "/ws",
		"ws_ticket_path":    "/api/ws-ticket",
		"sync_path":         "/api/messages/sync",
		"push_devices_path": "/api/push/devices",
		"health_path":       "/healthz",
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected fields: %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

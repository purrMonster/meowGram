package handler

import (
	"encoding/json"
	"net/http"

	"meowgram/server/internal/config"
)

// ClientConfigPath is fixed: a client fetches it before it knows anything else
// about the server, so it can't be an environment-configured path.
const ClientConfigPath = "/api/client-config"

// ClientConfigResponse is what a generic (domain-free) client build needs to
// sign in and connect. Everything here is public: the issuer and the public
// OAuth client ID are visible in any login redirect anyway.
type ClientConfigResponse struct {
	Issuer          string `json:"issuer"`
	ClientID        string `json:"client_id"`
	WSPath          string `json:"ws_path"`
	WSTicketPath    string `json:"ws_ticket_path"`
	SyncPath        string `json:"sync_path"`
	PushDevicesPath string `json:"push_devices_path"`
	HealthPath      string `json:"health_path"`
}

// ClientConfigHandler serves the server's sign-in and endpoint settings, so the
// published app and web image carry no domain of their own.
func ClientConfigHandler(cfg *config.Config) http.HandlerFunc {
	body, _ := json.Marshal(ClientConfigResponse{
		Issuer:          cfg.AutheliaIssuer,
		ClientID:        cfg.AutheliaClientID,
		WSPath:          cfg.WSEndpoint,
		WSTicketPath:    cfg.WSTicketEndpoint,
		SyncPath:        cfg.SyncEndpoint,
		PushDevicesPath: cfg.PushDeviceEndpoint,
		HealthPath:      cfg.HealthEndpoint,
	})
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// Clients cache it themselves; revalidate so issuer changes reach them.
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(body)
	}
}

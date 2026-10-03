package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"meowgram/server/internal/config"
)

// HealthResponse represents the payload returned by /healthz.
type HealthResponse struct {
	Status      string    `json:"status"`
	Service     string    `json:"service"`
	Domain      string    `json:"domain"`
	Environment string    `json:"environment"`
	Timestamp   time.Time `json:"timestamp"`
}

// HealthHandler returns a lightweight health-check handler.
func HealthHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		resp := HealthResponse{
			Status:      "ok",
			Service:     "meowgram-server",
			Domain:      cfg.AppDomain,
			Environment: cfg.Environment,
			Timestamp:   time.Now().UTC(),
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(resp)
	}
}

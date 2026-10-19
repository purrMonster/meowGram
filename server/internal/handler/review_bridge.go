package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/config"
	"net/http"
	"os"
	"strconv"
	"time"
)

// ReviewBridge must be wrapped in human OIDC middleware, never the bot gate.
// The actor comes only from that verified identity, not any client header/body.
func ReviewBridge(c config.Integration) (http.Handler, error) {
	key, err := os.ReadFile(c.ReviewKeyFile)
	key = bytes.TrimSpace(key) // Match tabbyTally's signing-key file convention.
	if err != nil || len(key) < 32 || len(key) > 4096 {
		return nil, errors.New("review signing key file must contain 32 to 4096 bytes")
	}
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return reviewBridge(c, key, client), nil
}

func reviewBridge(c config.Integration, key []byte, client *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		u, ok := auth.UserFromContext(r.Context())
		if !ok {
			http.Error(w, "human authentication required", 401)
			return
		}
		if !c.ReviewActors[u.AutheliaSub] {
			http.Error(w, "review access denied", 403)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
		if err != nil {
			http.Error(w, "invalid review request", 400)
			return
		}
		var body map[string]json.RawMessage
		if json.Unmarshal(raw, &body) != nil || body == nil {
			http.Error(w, "invalid review request", 400)
			return
		}
		allowed := map[string]bool{"action_id": true, "review_id": true, "revision": true, "action": true, "facts": true, "reason": true, "treatment": true}
		for k := range body {
			if !allowed[k] {
				http.Error(w, "unexpected review field", 400)
				return
			}
		}
		// The processing service is authoritative for full financial validation and
		// repeat/stale decisions. Forward exact bytes for stable request receipts.
		request, err := http.NewRequestWithContext(r.Context(), "POST", c.ReviewURL, bytes.NewReader(raw))
		if err != nil {
			http.Error(w, "review service unavailable", 503)
			return
		}
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(timestamp + "\n" + u.AutheliaSub + "\n"))
		mac.Write(raw)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Review-Timestamp", timestamp)
		request.Header.Set("X-Review-Actor", u.AutheliaSub)
		request.Header.Set("X-Review-Signature", hex.EncodeToString(mac.Sum(nil)))
		response, err := client.Do(request)
		if err != nil {
			http.Error(w, "review service unavailable", 503)
			return
		}
		defer response.Body.Close()
		result, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil || len(result) > 65536 {
			http.Error(w, "review service unavailable", 503)
			return
		}
		switch response.StatusCode {
		case 200:
			if !json.Valid(result) {
				http.Error(w, "review service unavailable", 503)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write(result)
		case 409:
			http.Error(w, "review changed or action invalid; refresh before deciding", 409)
		case 403:
			http.Error(w, "review service authorization failed", 503)
		case 429:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "review request throttled", 429)
		default:
			http.Error(w, "review service unavailable", 503)
		}
	})
}

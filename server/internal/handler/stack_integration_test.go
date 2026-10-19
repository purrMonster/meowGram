package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/config"
	"meowgram/server/internal/database"
	"meowgram/server/internal/model"
	"meowgram/server/internal/ratelimit"
	"meowgram/server/internal/repository"
)

type scratchDirectory struct{ id string }

func (d scratchDirectory) Authenticate(_ context.Context, user, password string) (botauth.Identity, error) {
	if user != "synthetic" || password != "synthetic" {
		return botauth.Identity{}, botauth.ErrCredentials
	}
	return botauth.Identity{Namespace: "scratch", ExternalID: d.id, DisplayName: "synthetic bot"}, nil
}

// Emulates the issuer/JWKS locally and exercises actual auth middleware, tickets,
// WebSocket persistence/broadcast, sync and device registration together.
func TestScratchAuthenticatedChatFlow(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("scratch PostgreSQL required")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := database.Open(databaseURL, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "scratch", Algorithm: "RS256", Use: "sig"}}})
	}))
	defer issuer.Close()
	cfg := &config.Config{AutheliaIssuer: issuer.URL, AutheliaJWKSURL: issuer.URL + "/jwks", AutheliaAudience: "scratch-api"}
	verifier, err := auth.NewOIDCVerifier(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	sign := func(audience string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": issuer.URL, "aud": audience, "sub": "scratch-stack-user", "preferred_username": "scratch", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix()})
		token.Header["kid"] = "scratch"
		signed, err := token.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}
	users := repository.NewUserRepository(db)
	messages := repository.NewMessageRepository(db)
	guard := auth.Middleware(verifier, users, logger)
	tickets := auth.NewWSTicketStore()
	hub := chat.NewHub(logger, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	mux := http.NewServeMux()
	mux.Handle("POST /ticket", guard(WSTicketHandler(tickets, logger)))
	mux.Handle("GET /sync", guard(SyncHandler(messages, logger)))
	mux.Handle("PUT /devices", guard(PushDeviceHandler(repository.NewDeviceRepository(db))))
	mux.Handle("DELETE /devices", guard(PushDeviceHandler(repository.NewDeviceRepository(db))))
	mux.Handle("/ws", WebSocketHandler(hub, messages, tickets, cfg, logger))
	server := httptest.NewServer(mux)
	defer server.Close()
	request := func(method, path, bearer, body string) *http.Response {
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+bearer)
		if strings.HasPrefix(bearer, "Basic ") {
			req.Header.Set("Authorization", bearer)
		}
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal("scratch HTTP request failed")
		}
		return res
	}
	invalid := request("POST", "/ticket", sign("wrong-audience"), "")
	invalid.Body.Close()
	if invalid.StatusCode != 401 {
		t.Fatal("wrong audience authenticated")
	}
	bearer := sign("scratch-api")
	res := request("POST", "/ticket", bearer, "")
	var ticket struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ticket); err != nil {
		t.Fatal("ticket response invalid")
	}
	res.Body.Close()
	if res.StatusCode != 200 || ticket.Ticket == "" {
		t.Fatal("ticket exchange failed")
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?ticket=" + url.QueryEscape(ticket.Ticket)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal("scratch WebSocket handshake failed")
	}
	defer conn.Close()
	replay := request("GET", "/ws?ticket="+url.QueryEscape(ticket.Ticket), bearer, "")
	replay.Body.Close()
	if replay.StatusCode != 401 {
		t.Fatal("ticket replay authenticated")
	}
	before := time.Now().Add(-time.Second)
	messageText := "scratch persisted message " + time.Now().UTC().Format(time.RFC3339Nano)
	if err := conn.WriteJSON(map[string]string{"text_content": messageText}); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var received model.WSMessage
	for received.ID == "" {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatal("scratch chat broadcast missing")
		}
		for _, line := range bytes.Split(data, []byte{'\n'}) {
			var frame model.WSMessage
			if err := json.Unmarshal(line, &frame); err != nil {
				t.Fatal(err)
			}
			if frame.Type == "chat" && frame.SenderID == "scratch-stack-user" && frame.TextContent == messageText {
				received = frame
				break
			}
		}
	}
	var text string
	if err := db.QueryRow(`SELECT text_content FROM messages WHERE id=$1`, received.ID).Scan(&text); err != nil || text != received.TextContent {
		t.Fatal("broadcast was not already persisted")
	}
	res = request("GET", "/sync?after="+url.QueryEscape(before.UTC().Format(time.RFC3339Nano)), bearer, "")
	var synced []model.Message
	if err := json.NewDecoder(res.Body).Decode(&synced); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	found := false
	for _, message := range synced {
		if message.ID == received.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("sync did not return persisted live message")
	}
	for _, method := range []string{"PUT", "DELETE"} {
		res = request(method, "/devices", bearer, `{"token":"scratch-stack-device"}`)
		res.Body.Close()
		if res.StatusCode != 204 {
			t.Fatalf("device %s failed", method)
		}
	}

	// Real OIDC guard protects the bridge; a TLS callback checks exact signed bytes.
	bridgeKey := []byte(strings.Repeat("k", 32))
	callback := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mac := hmac.New(sha256.New, bridgeKey)
		mac.Write([]byte(r.Header.Get("X-Review-Timestamp") + "\nscratch-stack-user\n"))
		mac.Write(raw)
		if r.Header.Get("X-Review-Actor") != "scratch-stack-user" || r.Header.Get("X-Review-Signature") != hex.EncodeToString(mac.Sum(nil)) || r.Header.Get("Authorization") != "" {
			t.Error("invalid signed bridge request")
			w.WriteHeader(403)
			return
		}
		_, _ = w.Write([]byte(`{"recorded":true}`))
	}))
	defer callback.Close()
	mux.Handle("POST /review", guard(reviewBridge(config.Integration{ReviewURL: callback.URL, ReviewActors: map[string]bool{"scratch-stack-user": true}}, bridgeKey, callback.Client())))
	for _, pair := range [][2]string{{"", "401"}, {sign("wrong-audience"), "401"}, {bearer, "200"}, {"Basic c3ludGhldGljOnN5bnRoZXRpYw==", "401"}} {
		r := request("POST", "/review", pair[0], `{"action_id":"test"}`)
		r.Body.Close()
		if (pair[1] == "200" && r.StatusCode != 200) || (pair[1] == "401" && r.StatusCode != 401) {
			t.Fatalf("review auth status %d", r.StatusCode)
		}
	}
	// The bot gate validates credentials again even for replayed delivery keys.
	bots := repository.NewBotRepository(db)
	gate := botauth.Gate{Directory: scratchDirectory{uuid.NewString()}, Resolve: func(ctx context.Context, id botauth.Identity) (botauth.Principal, error) {
		p, err := bots.Resolve(ctx, id.Namespace, id.ExternalID, id.DisplayName)
		if err != nil {
			return botauth.Principal{}, err
		}
		return botauth.Principal{ID: p.ID, DisplayName: p.Username}, nil
	}, Ingress: ratelimit.New(1000, 100), API: ratelimit.New(1000, 100), HumanOnly: guard, Logger: logger, SendPath: "/send", Timeout: time.Second}
	send, err := gate.Handler(SendHandler(hub.MessageService(messages)))
	if err != nil {
		t.Fatal(err)
	}
	botServer := httptest.NewTLSServer(send)
	defer botServer.Close()
	idempotency := uuid.NewString()
	receiptID := ""
	for i, tc := range []struct {
		text, password string
		status         int
	}{{"notice", "synthetic", 201}, {"notice", "synthetic", 200}, {"changed", "synthetic", 409}, {"notice", "wrong", 401}} {
		raw, _ := json.Marshal(map[string]string{"text_content": tc.text})
		req, _ := http.NewRequest("POST", botServer.URL+"/send", bytes.NewReader(raw))
		req.SetBasicAuth("synthetic", tc.password)
		req.Header.Set("Idempotency-Key", idempotency)
		res, err := botServer.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != tc.status {
			t.Fatalf("send %d got %d", i, res.StatusCode)
		}
		if res.StatusCode == 200 || res.StatusCode == 201 {
			var receipt struct {
				ID        string `json:"id"`
				Key       string `json:"idempotency_key"`
				Persisted bool   `json:"persisted"`
			}
			if json.NewDecoder(res.Body).Decode(&receipt) != nil || !receipt.Persisted || receipt.Key != idempotency || receipt.ID == "" {
				t.Fatal("invalid receipt")
			}
			if receiptID != "" && receipt.ID != receiptID {
				t.Fatal("duplicate notification")
			}
			receiptID = receipt.ID
		}
		res.Body.Close()
	}
}

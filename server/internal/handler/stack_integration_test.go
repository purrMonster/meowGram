package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
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
	"github.com/gorilla/websocket"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/chat"
	"meowgram/server/internal/config"
	"meowgram/server/internal/database"
	"meowgram/server/internal/model"
	"meowgram/server/internal/repository"
)

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
}

package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/config"
	"meowgram/server/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReviewBridgeActorSigningAndFailures(t *testing.T) {
	key := []byte(strings.Repeat("x", 32))
	count := 0
	status := 200
	backend := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		body, _ := io.ReadAll(r.Body)
		actor := r.Header.Get("X-Review-Actor")
		if actor != "owner" || r.Header.Get("Authorization") != "" {
			t.Error("untrusted actor or leaked bearer")
		}
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(r.Header.Get("X-Review-Timestamp") + "\nowner\n"))
		mac.Write(body)
		if hex.EncodeToString(mac.Sum(nil)) != r.Header.Get("X-Review-Signature") {
			t.Error("signature mismatch")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"result":"recorded"}`))
	}))
	defer backend.Close()
	client := backend.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	h := reviewBridge(config.Integration{ReviewURL: backend.URL, ReviewActors: map[string]bool{"owner": true}}, key, client)
	request := func(actor, body string) int {
		r := httptest.NewRequest("POST", "/review", strings.NewReader(body))
		r.Header.Set("X-Review-Actor", "forged")
		r.Header.Set("Authorization", "Bearer must-not-forward")
		if actor != "" {
			r = r.WithContext(context.WithValue(r.Context(), auth.UserContextKey, &model.User{AutheliaSub: actor}))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	for _, actor := range []string{"", "other"} {
		if code := request(actor, `{}`); code != 401 && code != 403 {
			t.Fatal(code)
		}
	}
	if count != 0 {
		t.Fatal("unauthorized request reached callback")
	}
	if request("owner", `{"actor":"forged"}`) != 400 {
		t.Fatal("actor body accepted")
	}
	if request("owner", `{"action_id":"retry-same-id"}`) != 200 {
		t.Fatal("valid callback failed")
	}
	for _, pair := range [][2]int{{409, 409}, {403, 503}, {429, 429}, {302, 503}, {500, 503}} {
		status = pair[0]
		if got := request("owner", `{}`); got != pair[1] {
			t.Fatalf("%d -> %d", status, got)
		}
	}
}

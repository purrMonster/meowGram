package handler

import (
	"context"
	"encoding/json"
	"io"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/config"
	"meowgram/server/internal/model"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Opt-in cross-project test runs the real Python review service and PostgreSQL.
// OIDC validation itself is tested by TestScratchAuthenticatedChatFlow.
func TestProcessingBridgeRoundTrip(t *testing.T) {
	target := os.Getenv("PROCESSING_BRIDGE_TEST_URL")
	if target == "" {
		t.Skip("run tabbyTally processing/compose.bridge.test.yaml")
	}
	fixtureURL := os.Getenv("PROCESSING_BRIDGE_FIXTURE_URL")
	fixture := func() map[string]any {
		res, err := http.Get(fixtureURL)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var data map[string]any
		if json.NewDecoder(res.Body).Decode(&data) != nil {
			t.Fatal("invalid fixture")
		}
		return data
	}
	initial := fixture()
	targetURL, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	// Emulate the TLS reverse proxy without changing the production callback code.
	tlsProxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(targetURL))
	defer tlsProxy.Close()
	h := reviewBridge(config.Integration{ReviewURL: tlsProxy.URL + "/review/actions", ReviewActors: map[string]bool{"owner": true}}, []byte(strings.Repeat("k", 32)), tlsProxy.Client())
	send := func(body string) (int, string) {
		time.Sleep(120 * time.Millisecond) // Real receiver's per-actor request budget.
		req := httptest.NewRequest("POST", "/review", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &model.User{AutheliaSub: "owner"}))
		req.Header.Set("X-Review-Actor", "forged")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		res := w.Result()
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(data)
	}
	action := map[string]any{"action_id": "cross-project-decision", "review_id": initial["review_id"], "revision": initial["revision"], "action": "ignore", "reason": "synthetic bridge acceptance"}
	raw, _ := json.Marshal(action)
	first, body := send(string(raw))
	if first != 200 {
		t.Fatalf("first decision: %d", first)
	}
	replay, replayBody := send(string(raw))
	var originalResult, replayedResult any
	if replay != 200 || json.Unmarshal([]byte(body), &originalResult) != nil || json.Unmarshal([]byte(replayBody), &replayedResult) != nil || !reflect.DeepEqual(originalResult, replayedResult) {
		t.Fatalf("replay did not return original receipt (status %d)", replay)
	}
	action["action_id"] = "stale-action"
	raw, _ = json.Marshal(action)
	if status, _ := send(string(raw)); status != 409 {
		t.Fatalf("stale decision: %d", status)
	}
	action["action_id"] = "cross-project-decision"
	action["reason"] = "conflicting retry"
	raw, _ = json.Marshal(action)
	if status, _ := send(string(raw)); status != 409 {
		t.Fatalf("conflicting decision: %d", status)
	}
	final := fixture()
	if final["decisions"] != float64(1) || final["state"] != "resolved" {
		t.Fatal("decision not committed exactly once")
	}
}

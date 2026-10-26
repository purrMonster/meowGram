package integration

import (
	"bytes"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"meowgram/server/internal/repository"
)

// Called after the original R2 assertions, including the deliberate LDAP outage.
// Independent human identities preserve the original fixture's rate budgets.
func managementAcceptance(t *testing.T, issuer string, key *rsa.PrivateKey, client *http.Client, ingress string, db *sql.DB) {
	t.Helper()
	tokens := map[string]string{}
	for _, actor := range []string{"management-a", "management-b", "management-c", "management-outsider"} {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": issuer, "aud": "scratch-api", "sub": actor, "preferred_username": actor, "exp": time.Now().Add(time.Hour).Unix()})
		token.Header["kid"] = "scratch"
		signed, err := token.SignedString(key)
		if err != nil {
			t.Fatal("management signing fixture failed")
		}
		tokens[actor] = signed
	}
	call := func(t *testing.T, want int, actor, method, path, body string) []byte {
		t.Helper()
		request, err := http.NewRequest(method, ingress+path, strings.NewReader(body))
		if err != nil {
			t.Fatal("management request construction failed")
		}
		if actor == "basic" {
			request.SetBasicAuth("synthetic-management-bot", "synthetic-invalid-password")
		} else {
			request.Header.Set("Authorization", "Bearer "+tokens[actor])
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal("management request failed (details suppressed)")
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err != nil {
			t.Fatal("management response read failed")
		}
		if response.StatusCode != want {
			t.Fatalf("management %s %s status=%d want=%d", method, path, response.StatusCode, want)
		}
		if response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("management response is cacheable")
		}
		return data
	}
	decode := func(t *testing.T, data []byte, target any) {
		t.Helper()
		if json.Unmarshal(data, target) != nil {
			t.Fatal("management response shape invalid")
		}
	}
	var group string
	t.Run("management_create_reuse_rename", func(t *testing.T) {
		for actor := range tokens {
			call(t, 200, actor, "GET", "/api/conversations", "")
		}
		body := `{"members":[{"kind":"human","id":"management-b"}]}`
		var started struct {
			Conversation repository.Conversation `json:"conversation"`
			Created      bool                    `json:"created"`
		}
		decode(t, call(t, 200, "management-a", "POST", "/api/conversations/start-chat", body), &started)
		if !started.Created || started.Conversation.ChannelID == nil {
			t.Fatal("start did not create group")
		}
		group = *started.Conversation.ChannelID
		decode(t, call(t, 200, "management-b", "POST", "/api/conversations/start-chat", `{"members":[{"kind":"human","id":"management-a"}]}`), &started)
		if started.Created || *started.Conversation.ChannelID != group {
			t.Fatal("typed set did not reuse group")
		}
		var named repository.Conversation
		decode(t, call(t, 201, "management-a", "POST", "/api/conversations", `{"name":"named","members":[{"kind":"human","id":"management-b"}]}`), &named)
		if named.ChannelID == nil || *named.ChannelID == group {
			t.Fatal("named group reused")
		}
		decode(t, call(t, 200, "management-b", "PATCH", "/api/conversations/"+group, `{"name":"  changed  "}`), &named)
		if named.Name == nil || *named.Name != "changed" {
			t.Fatal("rename not trimmed")
		}
		decode(t, call(t, 200, "management-b", "PATCH", "/api/conversations/"+group, `{"name":null}`), &named)
		if named.Name != nil {
			t.Fatal("clear name failed")
		}
	})
	if group == "" {
		t.Fatal("management group setup failed")
	}
	t.Run("management_members_picker_persistence", func(t *testing.T) {
		var before, after int64
		if db.QueryRow("SELECT membership_version FROM users WHERE authelia_sub='management-c'").Scan(&before) != nil {
			t.Fatal("revision query failed")
		}
		call(t, 204, "management-b", "POST", "/api/conversations/"+group+"/members", `{"kind":"human","id":"management-c"}`)
		if db.QueryRow("SELECT membership_version FROM users WHERE authelia_sub='management-c'").Scan(&after) != nil || after <= before {
			t.Fatal("membership access change not persisted")
		}
		call(t, 204, "management-a", "POST", "/api/conversations/"+group+"/members", `{"kind":"human","id":"management-c"}`)
		var unchanged int64
		if db.QueryRow("SELECT membership_version FROM users WHERE authelia_sub='management-c'").Scan(&unchanged) != nil || unchanged != after {
			t.Fatal("idempotent add changed revision")
		}
		var members repository.MemberPage
		decode(t, call(t, 200, "management-c", "GET", "/api/conversations/"+group+"/members", ""), &members)
		if len(members.Members) != 3 {
			t.Fatal("member list wrong")
		}
		var principals repository.PrincipalPage
		decode(t, call(t, 200, "management-c", "GET", "/api/principals?q=management", ""), &principals)
		if len(principals.Principals) < 4 {
			t.Fatal("picker missing provisioned humans")
		}
		call(t, 204, "management-c", "DELETE", "/api/conversations/"+group+"/members/human/management-b", "")
		denied := call(t, 403, "management-b", "GET", "/api/conversations/"+group+"/members", "")
		unknown := call(t, 403, "management-outsider", "GET", "/api/conversations/"+uuid.NewString()+"/members", "")
		if !bytes.Equal(denied, unknown) {
			t.Fatal("unknown group leaks different denial")
		}
	})
	t.Run("management_basic_401_during_directory_outage", func(t *testing.T) {
		for _, path := range []string{"/api/conversations", "/api/conversations/start-chat", "/api/conversations/" + group, "/api/conversations/" + group + "/members", "/api/conversations/" + group + "/members/human/management-a", "/api/conversations/" + group + "/leave", "/api/conversations/lounge/mute", "/api/conversations/lounge/read", "/api/principals?q=x"} {
			call(t, 401, "basic", "OPTIONS", path, "invalid JSON")
		}
	})
	t.Run("management_read_mute_leave", func(t *testing.T) {
		// Isolated final fixture row: no original send/sync assertions observe it.
		messageID := uuid.NewString()
		if _, err := db.Exec("INSERT INTO messages(id,sender_id,text_content,channel_id,created_at) VALUES ($1,'management-c','management read fixture',$2,'2020-01-01Z')", messageID, group); err != nil {
			t.Fatal("read fixture insert failed")
		}
		call(t, 204, "management-a", "PUT", "/api/conversations/"+group+"/mute", `{"muted":true}`)
		call(t, 204, "management-a", "PUT", "/api/conversations/"+group+"/read", `{"message_id":"`+messageID+`"}`)
		var page repository.ConversationPage
		decode(t, call(t, 200, "management-a", "GET", "/api/conversations", ""), &page)
		found := false
		for _, c := range page.Conversations {
			if c.ChannelID != nil && *c.ChannelID == group {
				found = true
				if !c.Muted || c.UnreadCount != 0 {
					t.Fatal("read/mute state wrong")
				}
			}
		}
		if !found {
			t.Fatal("group missing from conversation list")
		}
		var stored string
		var muted bool
		if db.QueryRow("SELECT read_message_id,muted FROM conversation_user_state WHERE user_sub='management-a' AND channel_id=$1", group).Scan(&stored, &muted) != nil || stored != messageID || !muted {
			t.Fatal("read/mute not persisted")
		}
		call(t, 204, "management-c", "POST", "/api/conversations/"+group+"/leave", "")
		call(t, 403, "management-c", "GET", "/api/conversations/"+group+"/members", "")
	})
}

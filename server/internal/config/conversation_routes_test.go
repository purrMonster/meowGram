package config

import "testing"

func TestConversationEndpointValidation(t *testing.T) {
	for _, tc := range []struct {
		c, p, reserved string
		bad            bool
	}{
		{"/api/conversations", "/api/principals", "/api/messages", false},
		{"/custom/chats", "/custom/people", "/api/messages", false},
		{"/api/conversations/alias", "/api/principals", "/api/messages", true},
		{"/api", "/api/principals", "/api/messages", true},
		{"/custom", "/custom/people", "/api/messages", true},
		{"/custom/chats", "/custom/people", "/custom/chats/secret", true},
		{"/custom/chats", "/custom/people", "/custom", true},
		{"/api/messages", "/custom/people", "/api/messages", true},
		{"/x%2Fy", "/custom/people", "/api/messages", true},
		{"/x/../y", "/custom/people", "/api/messages", true},
		{"/x/", "/custom/people", "/api/messages", true},
		{"/x//y", "/custom/people", "/api/messages", true},
		{"/x?y", "/custom/people", "/api/messages", true},
	} {
		t.Run(tc.c+tc.p+tc.reserved, func(t *testing.T) {
			err := ValidateConversationEndpoints(tc.c, tc.p, tc.reserved)
			if (err != nil) != tc.bad {
				t.Fatalf("err=%v want bad=%v", err, tc.bad)
			}
		})
	}
}
func TestConversationEndpointLoad(t *testing.T) {
	cleanEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("AUTHELIA_DOMAIN", "auth.example.home.arpa")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConversationsEndpoint != "/api/conversations" || cfg.PrincipalsEndpoint != "/api/principals" {
		t.Fatal("wrong defaults")
	}
	t.Setenv("CONVERSATIONS_ENDPOINT", "/custom/chats")
	cfg, err = Load()
	if err != nil || cfg.ConversationsEndpoint != "/custom/chats" {
		t.Fatalf("alias error %v", err)
	}
	t.Setenv("WS_ENDPOINT", "/custom/chats/stream")
	if _, err = Load(); err == nil {
		t.Fatal("accepted reserved route inside management subtree")
	}
}

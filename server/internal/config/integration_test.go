package config

import "testing"

func TestIntegrationRejectsExistingRoutes(t *testing.T) {
	for _, path := range []string{"/api/ws-ticket", "/custom-ticket", "/ws"} {
		for _, c := range []Integration{{SendPath: path}, {ReviewPath: path}} {
			if c.ValidateRoutes("/api/ws-ticket", "/custom-ticket", "/ws") == nil {
				t.Fatal("route collision accepted")
			}
		}
	}
	if (Integration{SendPath: "/api/messages", ReviewPath: "/api/review/actions"}).ValidateRoutes("/api/ws-ticket") != nil {
		t.Fatal("distinct routes rejected")
	}
}

func TestIntegrationOptInAndValidation(t *testing.T) {
	for _, key := range []string{"MESSAGE_SEND_ENDPOINT", "REVIEW_ACTION_ENDPOINT", "REVIEW_CALLBACK_URL", "REVIEW_SIGNING_KEY_FILE", "REVIEW_ALLOWED_SUBJECTS"} {
		t.Setenv(key, "")
	}
	c, err := LoadIntegration()
	if err != nil || c.SendPath != "" || c.ReviewPath != "" {
		t.Fatal("enabled by default")
	}
	t.Setenv("MESSAGE_SEND_ENDPOINT", "/api/messages")
	t.Setenv("REVIEW_ACTION_ENDPOINT", "/api/review/actions")
	t.Setenv("REVIEW_CALLBACK_URL", "https://example.home.arpa/review/actions")
	t.Setenv("REVIEW_SIGNING_KEY_FILE", "/run/secrets/review-key")
	t.Setenv("REVIEW_ALLOWED_SUBJECTS", "owner")
	if _, err = LoadIntegration(); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"MESSAGE_SEND_ENDPOINT", "/api/{wildcard}"}, {"REVIEW_ACTION_ENDPOINT", "/api/messages"}, {"REVIEW_CALLBACK_URL", "http://example.home.arpa/actions"}, {"REVIEW_CALLBACK_URL", "https://user:secret@example.home.arpa/actions"}, {"REVIEW_ALLOWED_SUBJECTS", ""}, {"REVIEW_SIGNING_KEY_FILE", ""}} {
		t.Run(pair[0]+pair[1], func(t *testing.T) {
			t.Setenv(pair[0], pair[1])
			if _, err := LoadIntegration(); err == nil {
				t.Fatal("unsafe setting accepted")
			}
		})
	}
}

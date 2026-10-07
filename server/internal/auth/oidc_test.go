package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"meowgram/server/internal/config"
)

type mockKeySet struct {
	payload []byte
	err     error
}

func (m *mockKeySet) VerifySignature(ctx context.Context, jwt string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.payload, nil
}

func TestOIDCVerifier_TrailingSlashAndAudience(t *testing.T) {
	cfg := &config.Config{
		AutheliaIssuer:   "https://auth.example.home.arpa",
		AutheliaDomain:   "auth.example.home.arpa",
		AppDomain:        "meow.example.home.arpa",
		AutheliaClientID: "meowgram",
	}

	rawClaims := map[string]interface{}{
		"iss":                "https://auth.example.home.arpa/", // Trailing slash from Authelia
		"sub":                "141a5dfa-67ef-4e4b-97e3-0599c1581451",
		"aud":                []string{"https://meow.example.home.arpa"},
		"exp":                time.Now().Add(1 * time.Hour).Unix(),
		"preferred_username": "sir_purrsalot",
		"email":              "purrs@example.home.arpa",
	}
	payload, err := json.Marshal(rawClaims)
	if err != nil {
		t.Fatalf("failed to marshal mock claims: %v", err)
	}

	verifier := NewOIDCVerifierWithKeySet(&mockKeySet{payload: payload}, cfg)

	claims, err := verifier.Verify(context.Background(), "mock.jwt.token")
	if err != nil {
		t.Fatalf("expected successful verification with trailing slash issuer, got err: %v", err)
	}

	if claims.Subject != "141a5dfa-67ef-4e4b-97e3-0599c1581451" {
		t.Errorf("expected sub '141a5dfa-67ef-4e4b-97e3-0599c1581451', got %s", claims.Subject)
	}
	if claims.PreferredUsername != "sir_purrsalot" {
		t.Errorf("expected preferred_username 'sir_purrsalot', got %s", claims.PreferredUsername)
	}

	resolved := resolveUsername(claims)
	if resolved != "sir_purrsalot" {
		t.Errorf("expected resolved username 'sir_purrsalot', got %s", resolved)
	}
}

func TestOIDCVerifier_FallbackUsernameHierarchy(t *testing.T) {
	cfg := &config.Config{
		AutheliaIssuer:   "https://auth.example.home.arpa",
		AppDomain:        "meow.example.home.arpa",
		AutheliaClientID: "meowgram",
	}

	tests := []struct {
		name             string
		claims           map[string]interface{}
		expectedUsername string
	}{
		{
			name: "fallback to name when preferred_username missing",
			claims: map[string]interface{}{
				"iss":   "https://auth.example.home.arpa",
				"sub":   "uuid-123",
				"aud":   "meowgram",
				"exp":   time.Now().Add(1 * time.Hour).Unix(),
				"name":  "Captain Whisker",
				"email": "captain@example.home.arpa",
			},
			expectedUsername: "Captain Whisker",
		},
		{
			name: "fallback to email prefix when name and preferred_username missing",
			claims: map[string]interface{}{
				"iss":   "https://auth.example.home.arpa",
				"sub":   "uuid-123",
				"aud":   "meowgram",
				"exp":   time.Now().Add(1 * time.Hour).Unix(),
				"email": "fluffy_cat@example.home.arpa",
			},
			expectedUsername: "fluffy_cat",
		},
		{
			name: "fallback to UUID sub when all display claims missing",
			claims: map[string]interface{}{
				"iss": "https://auth.example.home.arpa",
				"sub": "141a5dfa-uuid",
				"aud": "meowgram",
				"exp": time.Now().Add(1 * time.Hour).Unix(),
			},
			expectedUsername: "141a5dfa-uuid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(tt.claims)
			verifier := NewOIDCVerifierWithKeySet(&mockKeySet{payload: payload}, cfg)
			claims, err := verifier.Verify(context.Background(), "mock.token")
			if err != nil {
				t.Fatalf("unexpected verification error: %v", err)
			}
			resolved := resolveUsername(claims)
			if resolved != tt.expectedUsername {
				t.Errorf("expected %q, got %q", tt.expectedUsername, resolved)
			}
		})
	}
}

func TestOIDCVerifier_Rejections(t *testing.T) {
	cfg := &config.Config{
		AutheliaIssuer:   "https://auth.example.home.arpa",
		AppDomain:        "meow.example.home.arpa",
		AutheliaAudience: "https://meow.example.home.arpa",
	}

	t.Run("expired token", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"iss": "https://auth.example.home.arpa",
			"sub": "user-1",
			"aud": "https://meow.example.home.arpa",
			"exp": time.Now().Add(-2 * time.Hour).Unix(),
		})
		verifier := NewOIDCVerifierWithKeySet(&mockKeySet{payload: payload}, cfg)
		_, err := verifier.Verify(context.Background(), "mock.token")
		if err == nil {
			t.Fatal("expected error for expired token, got nil")
		}
	})

	t.Run("mismatched issuer", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"iss": "https://rogue-auth.example.com",
			"sub": "user-1",
			"aud": "https://meow.example.home.arpa",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		verifier := NewOIDCVerifierWithKeySet(&mockKeySet{payload: payload}, cfg)
		_, err := verifier.Verify(context.Background(), "mock.token")
		if err == nil {
			t.Fatal("expected error for mismatched issuer, got nil")
		}
	})

	t.Run("mismatched audience", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"iss": "https://auth.example.home.arpa",
			"sub": "user-1",
			"aud": "https://rogue-app.example.com",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
		})
		verifier := NewOIDCVerifierWithKeySet(&mockKeySet{payload: payload}, cfg)
		_, err := verifier.Verify(context.Background(), "mock.token")
		if err == nil {
			t.Fatal("expected error for mismatched audience, got nil")
		}
	})

	t.Run("invalid signature error", func(t *testing.T) {
		verifier := NewOIDCVerifierWithKeySet(&mockKeySet{err: errors.New("bad signature")}, cfg)
		_, err := verifier.Verify(context.Background(), "mock.token")
		if err == nil {
			t.Fatal("expected signature error, got nil")
		}
	})
}

func verifyClaims(t *testing.T, cfg *config.Config, claims map[string]interface{}) error {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	_, err = NewOIDCVerifierWithKeySet(&mockKeySet{payload: payload}, cfg).Verify(context.Background(), "mock.token")
	return err
}

func TestOIDCVerifier_MandatoryClaims(t *testing.T) {
	cfg := &config.Config{
		AutheliaIssuer:   "https://auth.example.home.arpa",
		AppDomain:        "meow.example.home.arpa",
		AutheliaClientID: "meowgram-client",
	}
	future := time.Now().Add(time.Hour).Unix()

	if err := verifyClaims(t, cfg, map[string]interface{}{
		"iss": "https://auth.example.home.arpa", "sub": "u1", "aud": "meowgram-client",
	}); err == nil {
		t.Error("token without exp must be rejected (it would never expire)")
	}
	if err := verifyClaims(t, cfg, map[string]interface{}{
		"iss": "https://auth.example.home.arpa", "sub": "u1", "exp": future,
	}); err == nil {
		t.Error("token without aud must be rejected")
	}
	if err := verifyClaims(t, cfg, map[string]interface{}{
		"iss": "https://auth.example.home.arpa", "sub": "u1", "exp": future, "aud": "meowgram-client",
	}); err != nil {
		t.Errorf("token for the configured client ID must be accepted: %v", err)
	}
	if err := verifyClaims(t, cfg, map[string]interface{}{
		"iss": "https://auth.example.home.arpa", "sub": "u1", "exp": future, "aud": "some-other-app",
	}); err == nil {
		t.Error("token for another client must be rejected")
	}
}

func TestOIDCVerifier_StrictConfiguredAudience(t *testing.T) {
	cfg := &config.Config{
		AutheliaIssuer:   "https://auth.example.home.arpa",
		AppDomain:        "meow.example.home.arpa",
		AutheliaClientID: "meowgram-client",
		AutheliaAudience: "https://meow.example.home.arpa",
	}
	future := time.Now().Add(time.Hour).Unix()

	for _, aud := range []string{"meowgram", "meowgram-client"} {
		if err := verifyClaims(t, cfg, map[string]interface{}{
			"iss": "https://auth.example.home.arpa", "sub": "u1", "exp": future, "aud": aud,
		}); err == nil {
			t.Errorf("aud %q must be rejected when AUTHELIA_AUDIENCE is configured", aud)
		}
	}
	if err := verifyClaims(t, cfg, map[string]interface{}{
		"iss": "https://auth.example.home.arpa", "sub": "u1", "exp": future,
		"aud": []string{"https://meow.example.home.arpa/"},
	}); err != nil {
		t.Errorf("configured audience (with trailing slash) must be accepted: %v", err)
	}
}

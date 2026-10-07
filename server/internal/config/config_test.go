package config

import (
	"testing"
)

// cleanEnv blanks every variable Load reads, so tests don't leak into each other
// (t.Setenv restores the previous values when the test ends).
func cleanEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"PORT", "HTTP_PORT", "APP_DOMAIN", "ENVIRONMENT", "APP_ENV", "LOG_LEVEL",
		"DATABASE_URL", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_DB",
		"AUTHELIA_DOMAIN", "AUTHELIA_ISSUER", "AUTHELIA_ISSUER_URL", "AUTHELIA_JWKS_URL",
		"AUTHELIA_CLIENT_ID", "OIDC_CLIENT_ID", "CLIENT_ID", "AUTHELIA_AUDIENCE", "OIDC_AUDIENCE",
		"SYNC_ENDPOINT", "MESSAGES_SYNC_ENDPOINT", "HEALTH_ENDPOINT", "WS_ENDPOINT", "WS_PATH",
		"IMMICH_DOMAIN", "IMMICH_API_URL", "CORS_ORIGINS", "GOOGLE_APPLICATION_CREDENTIALS",
	} {
		t.Setenv(k, "")
	}
}

func TestConfig_AutheliaDomainDerivation(t *testing.T) {
	cleanEnv(t)
	// Clean environment variables for test
	t.Setenv("AUTHELIA_ISSUER", "")
	t.Setenv("AUTHELIA_ISSUER_URL", "")
	t.Setenv("AUTHELIA_JWKS_URL", "")
	t.Setenv("AUTHELIA_DOMAIN", "")

	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("AUTHELIA_DOMAIN", "auth.example.home.arpa")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected Load to succeed with AUTHELIA_DOMAIN, got: %v", err)
	}

	if cfg.AutheliaDomain != "auth.example.home.arpa" {
		t.Errorf("expected AutheliaDomain to be auth.example.home.arpa, got: %s", cfg.AutheliaDomain)
	}

	if cfg.AutheliaIssuer != "https://auth.example.home.arpa" {
		t.Errorf("expected AutheliaIssuer to be https://auth.example.home.arpa, got: %s", cfg.AutheliaIssuer)
	}

	if cfg.AutheliaJWKSURL != "https://auth.example.home.arpa/jwks.json" {
		t.Errorf("expected AutheliaJWKSURL to be https://auth.example.home.arpa/jwks.json, got: %s", cfg.AutheliaJWKSURL)
	}

	if cfg.SyncEndpoint != "/api/messages/sync" {
		t.Errorf("expected default SyncEndpoint /api/messages/sync, got: %s", cfg.SyncEndpoint)
	}

	if cfg.HealthEndpoint != "/healthz" {
		t.Errorf("expected default HealthEndpoint /healthz, got: %s", cfg.HealthEndpoint)
	}

	if cfg.WSEndpoint != "/ws" {
		t.Errorf("expected default WSEndpoint /ws, got: %s", cfg.WSEndpoint)
	}
}

func TestConfig_LocalhostAutheliaDomain(t *testing.T) {
	cleanEnv(t)
	t.Setenv("AUTHELIA_ISSUER", "")
	t.Setenv("AUTHELIA_ISSUER_URL", "")
	t.Setenv("AUTHELIA_JWKS_URL", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("AUTHELIA_DOMAIN", "localhost:9091")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected Load to succeed, got: %v", err)
	}

	if cfg.AutheliaIssuer != "http://localhost:9091" {
		t.Errorf("expected http://localhost:9091 for local domain, got: %s", cfg.AutheliaIssuer)
	}

	if cfg.AutheliaJWKSURL != "http://localhost:9091/jwks.json" {
		t.Errorf("expected http://localhost:9091/jwks.json, got: %s", cfg.AutheliaJWKSURL)
	}
}

func TestConfig_CustomEndpoints(t *testing.T) {
	cleanEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("AUTHELIA_ISSUER", "http://localhost:9091")
	t.Setenv("SYNC_ENDPOINT", "v1/custom/sync")
	t.Setenv("HEALTH_ENDPOINT", "ping")
	t.Setenv("WS_ENDPOINT", "chat/socket")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected Load to succeed, got: %v", err)
	}

	if cfg.SyncEndpoint != "/v1/custom/sync" {
		t.Errorf("expected /v1/custom/sync, got: %s", cfg.SyncEndpoint)
	}

	if cfg.HealthEndpoint != "/ping" {
		t.Errorf("expected /ping, got: %s", cfg.HealthEndpoint)
	}

	if cfg.WSEndpoint != "/chat/socket" {
		t.Errorf("expected /chat/socket, got: %s", cfg.WSEndpoint)
	}
}

func TestConfig_DefaultsHaveNoHardcodedDomainsAndMatchClientID(t *testing.T) {
	cleanEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("AUTHELIA_ISSUER", "http://localhost:9091")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ImmichDomain != "" || cfg.ImmichAPIURL != "" {
		t.Errorf("Immich must have no hardcoded default, got %q / %q", cfg.ImmichDomain, cfg.ImmichAPIURL)
	}
	if cfg.AutheliaClientID != "meowgram-client" {
		t.Errorf("default client ID must match the Flutter client, got %q", cfg.AutheliaClientID)
	}
}

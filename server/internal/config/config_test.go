package config

import (
	"os"
	"testing"
)

func TestConfig_AutheliaDomainDerivation(t *testing.T) {
	// Clean environment variables for test
	os.Unsetenv("AUTHELIA_ISSUER")
	os.Unsetenv("AUTHELIA_ISSUER_URL")
	os.Unsetenv("AUTHELIA_JWKS_URL")
	os.Unsetenv("AUTHELIA_DOMAIN")

	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("AUTHELIA_DOMAIN", "auth.purrbrews.cc")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected Load to succeed with AUTHELIA_DOMAIN, got: %v", err)
	}

	if cfg.AutheliaDomain != "auth.purrbrews.cc" {
		t.Errorf("expected AutheliaDomain to be auth.purrbrews.cc, got: %s", cfg.AutheliaDomain)
	}

	if cfg.AutheliaIssuer != "https://auth.purrbrews.cc" {
		t.Errorf("expected AutheliaIssuer to be https://auth.purrbrews.cc, got: %s", cfg.AutheliaIssuer)
	}

	if cfg.AutheliaJWKSURL != "https://auth.purrbrews.cc/jwks.json" {
		t.Errorf("expected AutheliaJWKSURL to be https://auth.purrbrews.cc/jwks.json, got: %s", cfg.AutheliaJWKSURL)
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
	os.Unsetenv("AUTHELIA_ISSUER")
	os.Unsetenv("AUTHELIA_ISSUER_URL")
	os.Unsetenv("AUTHELIA_JWKS_URL")
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("AUTHELIA_DOMAIN", "localhost:9091")

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
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("AUTHELIA_ISSUER", "http://localhost:9091")
	os.Setenv("SYNC_ENDPOINT", "v1/custom/sync")
	os.Setenv("HEALTH_ENDPOINT", "ping")
	os.Setenv("WS_ENDPOINT", "chat/socket")

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

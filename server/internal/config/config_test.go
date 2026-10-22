package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// cleanEnv blanks every variable Load reads, so tests don't leak into each other
// (t.Setenv restores the previous values when the test ends).
func cleanEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"MESSAGE_SEND_ENDPOINT", "IDEMPOTENCY_RETENTION", "BOT_BASIC_AUTH_ENABLED", "PORT", "HTTP_PORT", "APP_DOMAIN", "ENVIRONMENT", "APP_ENV", "LOG_LEVEL",
		"DATABASE_URL", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_DB",
		"AUTHELIA_DOMAIN", "AUTHELIA_ISSUER", "AUTHELIA_ISSUER_URL", "AUTHELIA_JWKS_URL",
		"AUTHELIA_CLIENT_ID", "OIDC_CLIENT_ID", "CLIENT_ID", "AUTHELIA_AUDIENCE", "OIDC_AUDIENCE",
		"SYNC_ENDPOINT", "MESSAGES_SYNC_ENDPOINT", "HEALTH_ENDPOINT", "WS_ENDPOINT", "WS_PATH", "WS_TICKET_ENDPOINT",
		"IMMICH_DOMAIN", "IMMICH_API_URL", "CORS_ORIGINS", "GOOGLE_APPLICATION_CREDENTIALS",
		"PUSH_DEVICE_ENDPOINT", "API_RATE_PER_MINUTE", "API_RATE_BURST",
		"MESSAGE_RATE_PER_MINUTE", "MESSAGE_RATE_BURST", "GLOBAL_RATE_PER_MINUTE", "GLOBAL_RATE_BURST",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("AUTHELIA_AUDIENCE", "http://localhost:8080")
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
	if cfg.WSTicketEndpoint != "/api/ws-ticket" {
		t.Errorf("expected default WSTicketEndpoint /api/ws-ticket, got: %s", cfg.WSTicketEndpoint)
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

func TestConfig_LocalhostLookalikeUsesHTTPS(t *testing.T) {
	cleanEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("AUTHELIA_DOMAIN", "localhost.attacker.example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected Load to succeed, got: %v", err)
	}
	if cfg.AutheliaIssuer != "https://localhost.attacker.example" {
		t.Fatalf("expected HTTPS for non-loopback hostname, got: %s", cfg.AutheliaIssuer)
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
	if cfg.AutheliaClientID != "meowgram" {
		t.Errorf("default client ID must match the Flutter client, got %q", cfg.AutheliaClientID)
	}
}

func TestConfig_OriginPolicyUsesConfiguredOrigins(t *testing.T) {
	cfg := &Config{
		AppDomain:   "chat.example.home.arpa",
		CORSOrigins: []string{"https://chat.example.home.arpa"},
	}
	if !cfg.IsAllowedOrigin("https://chat.example.home.arpa") {
		t.Fatal("configured origin should be allowed")
	}
	if cfg.IsAllowedOrigin("http://chat.example.home.arpa") {
		t.Fatal("an unconfigured insecure origin must not be implicitly allowed")
	}
}

func TestConfig_ProductionOriginDefaultsUseHTTPS(t *testing.T) {
	cleanEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("AUTHELIA_ISSUER", "https://auth.example.home.arpa")
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("APP_DOMAIN", "chat.example.home.arpa")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.IsAllowedOrigin("https://chat.example.home.arpa") {
		t.Fatal("production default should allow the configured HTTPS app origin")
	}
	if cfg.IsAllowedOrigin("http://chat.example.home.arpa") {
		t.Fatal("production default must not allow plaintext HTTP")
	}
}

func TestLoadDotenv_ExplicitEmptyEnvironmentValueWins(t *testing.T) {
	t.Setenv("DOTENV_EMPTY_OVERRIDE_TEST", "")
	workingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workingDir, ".env"), []byte("DOTENV_EMPTY_OVERRIDE_TEST=from-file\n"), 0600); err != nil {
		t.Fatalf("write dotenv fixture: %v", err)
	}
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	loadDotenv()
	if got := os.Getenv("DOTENV_EMPTY_OVERRIDE_TEST"); got != "" {
		t.Fatalf("explicit empty environment value was overwritten by dotenv: %q", got)
	}
}

func TestRateLimitConfiguration(t *testing.T) {
	for _, raw := range []string{"0", "-1", "nope", "1000001"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("API_RATE_PER_MINUTE", raw)
			if _, err := loadRateLimits(); err == nil {
				t.Fatal("invalid rate accepted")
			}
		})
	}
	t.Setenv("API_RATE_PER_MINUTE", "240")
	limits, err := loadRateLimits()
	if err != nil || limits.APIPerMinute != 240 {
		t.Fatal("rate override ignored")
	}
}

func TestHTTPSendConfiguration(t *testing.T) {
	cleanEnv(t)
	t.Setenv("AUTHELIA_ISSUER", "http://localhost:9091")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SendEndpoint != "/api/messages" || cfg.IdempotencyRetention != 168*time.Hour || cfg.BotAuth.Enabled {
		t.Fatal("unsafe send defaults")
	}
	for _, bad := range []string{"0h", "167h", "8761h", "invalid"} {
		t.Setenv("IDEMPOTENCY_RETENTION", bad)
		if _, err := Load(); err == nil {
			t.Fatal("invalid retention accepted")
		}
	}
	t.Setenv("IDEMPOTENCY_RETENTION", "168h")
	for _, bad := range []string{"/api/ws-ticket", "/ws", "/api/push/devices", "/bad/{id}", "/bad/", "/bad?x"} {
		t.Setenv("MESSAGE_SEND_ENDPOINT", bad)
		if _, err := Load(); err == nil {
			t.Fatal("unsafe route accepted")
		}
	}
	t.Setenv("MESSAGE_SEND_ENDPOINT", "/custom/send")
	t.Setenv("BOT_BASIC_AUTH_ENABLED", "true")
	t.Setenv("API_RATE_PER_MINUTE", "29")
	if _, err := Load(); err == nil {
		t.Fatal("bot budget below polling contract accepted")
	}
	t.Setenv("API_RATE_PER_MINUTE", "30")
	t.Setenv("GLOBAL_RATE_PER_MINUTE", "60")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}

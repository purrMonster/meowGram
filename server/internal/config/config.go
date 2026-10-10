package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents runtime configuration loaded strictly from environment variables and .env files.
type Config struct {
	Port               string
	AppDomain          string
	CORSOrigins        []string
	Environment        string
	LogLevel           string
	DatabaseURL        string
	AutheliaDomain     string
	AutheliaIssuer     string
	AutheliaJWKSURL    string
	AutheliaClientID   string
	AutheliaAudience   string
	GoogleCredentials  string
	SyncEndpoint       string
	HealthEndpoint     string
	WSEndpoint         string
	WSTicketEndpoint   string
	PushDeviceEndpoint string
	ImmichDomain       string
	ImmichAPIURL       string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
}

// Load populates and validates Config from process environment and candidate .env files.
func Load() (*Config, error) {
	// 1. Attempt loading from candidate .env files if present
	loadDotenv()

	// Port fallback precedence: PORT -> HTTP_PORT -> 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("HTTP_PORT")
	}
	if port == "" {
		port = "8080"
	}

	// App domain strictly from environment (defaulting to localhost for local dev fallback)
	appDomain := os.Getenv("APP_DOMAIN")
	if appDomain == "" {
		appDomain = "localhost"
	}

	// Environment (development, staging, production)
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = os.Getenv("APP_ENV")
	}
	if env == "" {
		env = "development"
	}

	// Log level (debug, info, warn, error)
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	// Database connection URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		pgUser := os.Getenv("POSTGRES_USER")
		pgPass := os.Getenv("POSTGRES_PASSWORD")
		pgHost := os.Getenv("POSTGRES_HOST")
		if pgHost == "" {
			pgHost = "localhost"
		}
		pgPort := os.Getenv("POSTGRES_PORT")
		if pgPort == "" {
			pgPort = "5432"
		}
		pgDB := os.Getenv("POSTGRES_DB")
		if pgUser != "" && pgDB != "" {
			dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", pgUser, pgPass, pgHost, pgPort, pgDB)
		}
	}
	if dbURL == "" {
		return nil, errors.New("database connection configuration is required (DATABASE_URL or POSTGRES_USER/POSTGRES_DB)")
	}

	// Authelia OIDC configurations (supports AUTHELIA_DOMAIN, AUTHELIA_ISSUER, AUTHELIA_ISSUER_URL)
	autheliaDomain := os.Getenv("AUTHELIA_DOMAIN")
	autheliaIssuer := os.Getenv("AUTHELIA_ISSUER")
	if autheliaIssuer == "" {
		autheliaIssuer = os.Getenv("AUTHELIA_ISSUER_URL")
	}

	// Automatically derive Authelia issuer from AUTHELIA_DOMAIN if not explicitly specified
	if autheliaIssuer == "" && autheliaDomain != "" {
		if strings.HasPrefix(autheliaDomain, "http://") || strings.HasPrefix(autheliaDomain, "https://") {
			autheliaIssuer = autheliaDomain
		} else if isLoopbackDomain(autheliaDomain) {
			autheliaIssuer = "http://" + autheliaDomain
		} else {
			autheliaIssuer = "https://" + autheliaDomain
		}
	}

	if autheliaIssuer == "" {
		return nil, errors.New("environment variable AUTHELIA_ISSUER or AUTHELIA_DOMAIN is required")
	}
	autheliaIssuer = strings.TrimRight(strings.TrimSpace(autheliaIssuer), "/")

	autheliaJWKSURL := os.Getenv("AUTHELIA_JWKS_URL")
	if autheliaJWKSURL == "" {
		autheliaJWKSURL = autheliaIssuer + "/jwks.json"
	}

	// Authelia Client ID & Expected Audience
	autheliaClientID := os.Getenv("AUTHELIA_CLIENT_ID")
	if autheliaClientID == "" {
		autheliaClientID = os.Getenv("OIDC_CLIENT_ID")
	}
	if autheliaClientID == "" {
		autheliaClientID = os.Getenv("CLIENT_ID")
	}
	if autheliaClientID == "" {
		// Matches the public client registered in Authelia and used by the Flutter app.
		autheliaClientID = "meowgram"
	}

	autheliaAudience := os.Getenv("AUTHELIA_AUDIENCE")
	if autheliaAudience == "" {
		autheliaAudience = os.Getenv("OIDC_AUDIENCE")
	}
	if strings.TrimSpace(autheliaAudience) == "" {
		return nil, errors.New("AUTHELIA_AUDIENCE is required (use the API audience configured for this app)")
	}
	autheliaAudience = strings.TrimSpace(autheliaAudience)

	// Sourcing REST & WebSocket Service Endpoints
	syncEndpoint := os.Getenv("SYNC_ENDPOINT")
	if syncEndpoint == "" {
		syncEndpoint = os.Getenv("MESSAGES_SYNC_ENDPOINT")
	}
	if syncEndpoint == "" {
		syncEndpoint = "/api/messages/sync"
	}
	if !strings.HasPrefix(syncEndpoint, "/") {
		syncEndpoint = "/" + syncEndpoint
	}

	healthEndpoint := os.Getenv("HEALTH_ENDPOINT")
	if healthEndpoint == "" {
		healthEndpoint = "/healthz"
	}
	if !strings.HasPrefix(healthEndpoint, "/") {
		healthEndpoint = "/" + healthEndpoint
	}

	wsEndpoint := os.Getenv("WS_ENDPOINT")
	if wsEndpoint == "" {
		wsEndpoint = os.Getenv("WS_PATH")
	}
	if wsEndpoint == "" {
		wsEndpoint = "/ws"
	}
	if !strings.HasPrefix(wsEndpoint, "/") {
		wsEndpoint = "/" + wsEndpoint
	}

	wsTicketEndpoint := os.Getenv("WS_TICKET_ENDPOINT")
	if wsTicketEndpoint == "" {
		wsTicketEndpoint = "/api/ws-ticket"
	}
	if !strings.HasPrefix(wsTicketEndpoint, "/") {
		wsTicketEndpoint = "/" + wsTicketEndpoint
	}

	// Immich Endpoints (Release 2). No default: zero-hardcoding rule.
	immichDomain := os.Getenv("IMMICH_DOMAIN")
	immichAPIURL := os.Getenv("IMMICH_API_URL")
	if immichAPIURL == "" && immichDomain != "" {
		immichAPIURL = "https://" + immichDomain + "/api"
	}

	// CORS Origins: comma-separated list of origins
	corsRaw := os.Getenv("CORS_ORIGINS")
	var corsOrigins []string
	if corsRaw != "" {
		for _, o := range strings.Split(corsRaw, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				corsOrigins = append(corsOrigins, trimmed)
			}
		}
	}

	// Default fallback origins if none specified
	if len(corsOrigins) == 0 {
		if env == "production" || env == "staging" {
			corsOrigins = []string{fmt.Sprintf("https://%s", appDomain)}
		} else {
			corsOrigins = []string{
				fmt.Sprintf("http://%s", appDomain),
				fmt.Sprintf("http://%s:%s", appDomain, port),
				"http://localhost",
				"http://localhost:8080",
				"http://localhost:8081",
				"http://localhost:3000",
				"http://127.0.0.1",
				"http://127.0.0.1:8080",
				"http://127.0.0.1:8081",
			}
		}
	}

	readTimeout := parseDurationSeconds(os.Getenv("READ_TIMEOUT_SECONDS"), 15*time.Second)
	writeTimeout := parseDurationSeconds(os.Getenv("WRITE_TIMEOUT_SECONDS"), 15*time.Second)
	idleTimeout := parseDurationSeconds(os.Getenv("IDLE_TIMEOUT_SECONDS"), 60*time.Second)

	googleCredentials := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	return &Config{
		Port:               port,
		AppDomain:          appDomain,
		CORSOrigins:        corsOrigins,
		Environment:        env,
		LogLevel:           logLevel,
		DatabaseURL:        dbURL,
		AutheliaDomain:     autheliaDomain,
		AutheliaIssuer:     autheliaIssuer,
		AutheliaJWKSURL:    autheliaJWKSURL,
		AutheliaClientID:   autheliaClientID,
		AutheliaAudience:   autheliaAudience,
		SyncEndpoint:       syncEndpoint,
		HealthEndpoint:     healthEndpoint,
		WSEndpoint:         wsEndpoint,
		WSTicketEndpoint:   wsTicketEndpoint,
		PushDeviceEndpoint: normalizePath(os.Getenv("PUSH_DEVICE_ENDPOINT"), "/api/push/devices"),
		ImmichDomain:       immichDomain,
		ImmichAPIURL:       immichAPIURL,
		GoogleCredentials:  googleCredentials,
		ReadTimeout:        readTimeout,
		WriteTimeout:       writeTimeout,
		IdleTimeout:        idleTimeout,
	}, nil
}

func normalizePath(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if !strings.HasPrefix(value, "/") {
		return "/" + value
	}
	return value
}

func isLoopbackDomain(value string) bool {
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		value = parsed.Host
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	if strings.EqualFold(value, "localhost") {
		return true
	}
	ip := net.ParseIP(value)
	return ip != nil && ip.IsLoopback()
}

// Address returns the listen address string.
func (c *Config) Address() string {
	return ":" + c.Port
}

// IsAllowedOrigin accepts only explicitly configured origins.
func (c *Config) IsAllowedOrigin(origin string) bool {
	if origin == "" {
		return true // Allow requests without an origin (e.g. mobile apps, curl, non-browser clients)
	}

	origin = strings.TrimRight(origin, "/")
	for _, allowed := range c.CORSOrigins {
		if strings.EqualFold(origin, strings.TrimRight(allowed, "/")) {
			return true
		}
	}

	return false
}

func parseDurationSeconds(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	sec, err := strconv.Atoi(raw)
	if err != nil || sec <= 0 {
		return fallback
	}
	return time.Duration(sec) * time.Second
}

package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents runtime configuration loaded strictly from environment variables.
type Config struct {
	Port            string
	AppDomain       string
	CORSOrigins     []string
	Environment     string
	LogLevel        string
	DatabaseURL     string
	AutheliaIssuer  string
	AutheliaJWKSURL string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
}

// Load populates and validates Config from environment variables.
func Load() (*Config, error) {
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

	// Authelia OIDC configurations (Mandatory contract)
	autheliaIssuer := os.Getenv("AUTHELIA_ISSUER")
	if autheliaIssuer == "" {
		return nil, errors.New("environment variable AUTHELIA_ISSUER is required")
	}

	autheliaJWKSURL := os.Getenv("AUTHELIA_JWKS_URL")
	if autheliaJWKSURL == "" {
		return nil, errors.New("environment variable AUTHELIA_JWKS_URL is required")
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
		corsOrigins = []string{
			fmt.Sprintf("http://%s", appDomain),
			fmt.Sprintf("http://%s:%s", appDomain, port),
			fmt.Sprintf("https://%s", appDomain),
			"http://localhost",
			"http://localhost:8080",
			"http://localhost:3000",
			"http://127.0.0.1",
			"http://127.0.0.1:8080",
		}
	}

	readTimeout := parseDurationSeconds(os.Getenv("READ_TIMEOUT_SECONDS"), 15*time.Second)
	writeTimeout := parseDurationSeconds(os.Getenv("WRITE_TIMEOUT_SECONDS"), 15*time.Second)
	idleTimeout := parseDurationSeconds(os.Getenv("IDLE_TIMEOUT_SECONDS"), 60*time.Second)

	return &Config{
		Port:            port,
		AppDomain:       appDomain,
		CORSOrigins:     corsOrigins,
		Environment:     env,
		LogLevel:        logLevel,
		DatabaseURL:     dbURL,
		AutheliaIssuer:  autheliaIssuer,
		AutheliaJWKSURL: autheliaJWKSURL,
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		IdleTimeout:     idleTimeout,
	}, nil
}

// Address returns the listen address string.
func (c *Config) Address() string {
	return ":" + c.Port
}

// IsAllowedOrigin checks if the provided origin header matches configured origins or app domain.
func (c *Config) IsAllowedOrigin(origin string) bool {
	if origin == "" {
		return true // Allow non-browser clients (native mobile/desktop)
	}

	normalized := strings.TrimRight(strings.ToLower(origin), "/")

	for _, allowed := range c.CORSOrigins {
		allowedNorm := strings.TrimRight(strings.ToLower(allowed), "/")
		if allowedNorm == "*" || allowedNorm == normalized {
			return true
		}
		if strings.Contains(allowedNorm, c.AppDomain) && strings.Contains(normalized, c.AppDomain) {
			return true
		}
	}

	return false
}

func parseDurationSeconds(val string, fallback time.Duration) time.Duration {
	if val == "" {
		return fallback
	}
	sec, err := strconv.Atoi(val)
	if err != nil || sec <= 0 {
		return fallback
	}
	return time.Duration(sec) * time.Second
}

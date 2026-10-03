package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"meowgram/server/internal/config"
	"meowgram/server/internal/model"
	"meowgram/server/internal/repository"

	"github.com/coreos/go-oidc/v3/oidc"
)

type contextKey string

const (
	UserContextKey contextKey = "meowgram.auth.user"
	SubContextKey  contextKey = "meowgram.auth.sub"
)

// AutheliaClaims captures standard OIDC identity fields from Authelia tokens.
type AutheliaClaims struct {
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Email             string `json:"email"`
}

// OIDCVerifier wraps token verification against Authelia's JWKS and Issuer.
type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
	issuer   string
}

// NewOIDCVerifier initializes an OIDC token verifier using Authelia's JWKS URL and Issuer.
func NewOIDCVerifier(ctx context.Context, cfg *config.Config) (*OIDCVerifier, error) {
	if cfg.AutheliaIssuer == "" || cfg.AutheliaJWKSURL == "" {
		return nil, errors.New("authelia issuer and JWKS URL must be configured")
	}

	keySet := oidc.NewRemoteKeySet(ctx, cfg.AutheliaJWKSURL)
	verifier := oidc.NewVerifier(cfg.AutheliaIssuer, keySet, &oidc.Config{
		SkipClientIDCheck: true,
	})

	return &OIDCVerifier{
		verifier: verifier,
		issuer:   cfg.AutheliaIssuer,
	}, nil
}

// Verify validates the raw token string and extracts Authelia claims.
func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (*AutheliaClaims, error) {
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	var claims AutheliaClaims
	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract token claims: %w", err)
	}

	if claims.Subject == "" {
		return nil, errors.New("token is missing mandatory 'sub' claim")
	}

	return &claims, nil
}

// Middleware enforces Authelia OIDC authentication and auto-provisions user records.
func Middleware(verifier *OIDCVerifier, userRepo *repository.UserRepository, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := extractToken(r)
			if rawToken == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing authentication token (Authorization header or ?token= query parameter required)")
				return
			}

			claims, err := verifier.Verify(r.Context(), rawToken)
			if err != nil {
				logger.Warn("OIDC token verification failed", "error", err, "remote_addr", r.RemoteAddr)
				writeAuthError(w, http.StatusUnauthorized, "invalid or expired authentication token")
				return
			}

			// Determine preferred username fallback hierarchy
			preferredUsername := resolveUsername(claims)

			// Auto-provision user record in PostgreSQL if not already present
			user, err := userRepo.GetOrCreateBySub(r.Context(), claims.Subject, preferredUsername)
			if err != nil {
				logger.Error("Failed to auto-provision user from OIDC identity",
					"sub", claims.Subject,
					"error", err,
				)
				writeAuthError(w, http.StatusInternalServerError, "failed to provision user identity")
				return
			}

			logger.Debug("Authenticated OIDC identity",
				"sub", claims.Subject,
				"user_id", user.ID,
				"username", user.Username,
			)

			// Inject user and sub claim into request context
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			ctx = context.WithValue(ctx, SubContextKey, claims.Subject)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves the authenticated User model from request context.
func UserFromContext(ctx context.Context) (*model.User, bool) {
	u, ok := ctx.Value(UserContextKey).(*model.User)
	return u, ok && u != nil
}

// SubFromContext retrieves the Authelia subject claim from request context.
func SubFromContext(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(SubContextKey).(string)
	return s, ok && s != ""
}

func extractToken(r *http.Request) string {
	// 1. Check Authorization Bearer header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	// 2. Check 'token' query parameter (Standard for WebSocket handshakes)
	queryToken := r.URL.Query().Get("token")
	if queryToken != "" {
		return strings.TrimSpace(queryToken)
	}

	return ""
}

func resolveUsername(claims *AutheliaClaims) string {
	if strings.TrimSpace(claims.PreferredUsername) != "" {
		return strings.TrimSpace(claims.PreferredUsername)
	}
	if strings.TrimSpace(claims.Name) != "" {
		return strings.TrimSpace(claims.Name)
	}
	if strings.TrimSpace(claims.Email) != "" {
		email := strings.TrimSpace(claims.Email)
		parts := strings.Split(email, "@")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
	}
	return claims.Subject
}

func writeAuthError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "unauthorized",
		"message": message,
	})
}

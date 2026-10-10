package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

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

// OIDCVerifier wraps token verification against Authelia's JWKS, Issuer, and Audience.
type OIDCVerifier struct {
	keySet         oidc.KeySet
	issuer         string
	autheliaDomain string
	audience       string
}

// rawTokenClaims models the complete set of standard claims in Authelia RS256 tokens.
type rawTokenClaims struct {
	Issuer            string          `json:"iss"`
	Subject           string          `json:"sub"`
	Audience          json.RawMessage `json:"aud"`
	ExpiresAt         int64           `json:"exp"`
	NotBefore         int64           `json:"nbf"`
	IssuedAt          int64           `json:"iat"`
	PreferredUsername string          `json:"preferred_username"`
	Name              string          `json:"name"`
	Email             string          `json:"email"`
}

// NewOIDCVerifier initializes an OIDC token verifier using Authelia's JWKS URL and Issuer.
func NewOIDCVerifier(ctx context.Context, cfg *config.Config) (*OIDCVerifier, error) {
	if cfg.AutheliaIssuer == "" || cfg.AutheliaJWKSURL == "" {
		return nil, errors.New("authelia issuer and JWKS URL must be configured")
	}

	keySet := oidc.NewRemoteKeySet(ctx, cfg.AutheliaJWKSURL)
	return NewOIDCVerifierWithKeySet(keySet, cfg), nil
}

// NewOIDCVerifierWithKeySet constructs an OIDCVerifier with an explicit KeySet (for production or mock testing).
func NewOIDCVerifierWithKeySet(keySet oidc.KeySet, cfg *config.Config) *OIDCVerifier {
	return &OIDCVerifier{
		keySet:         keySet,
		issuer:         cfg.AutheliaIssuer,
		autheliaDomain: cfg.AutheliaDomain,
		audience:       cfg.AutheliaAudience,
	}
}

// Verify validates the raw RS256 token signature against JWKS and validates issuer, audience, and expiry.
func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (*AutheliaClaims, error) {
	if rawToken == "" {
		return nil, errors.New("empty token")
	}

	// 1. Verify cryptographic RS256 signature against Authelia JWKS
	payload, err := v.keySet.VerifySignature(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("cryptographic signature verification failed: %w", err)
	}

	// 2. Unmarshal payload claims
	var claims rawTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to extract token claims: %w", err)
	}

	// 3. Validate mandatory subject
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, errors.New("token is missing mandatory 'sub' claim")
	}

	// 4. Validate expiration (mandatory) with 1-minute clock skew tolerance
	now := time.Now()
	if claims.ExpiresAt <= 0 {
		return nil, errors.New("token is missing mandatory 'exp' claim")
	}
	if now.Add(-1*time.Minute).Unix() > claims.ExpiresAt {
		return nil, errors.New("token has expired")
	}
	if claims.NotBefore > 0 {
		if now.Add(1*time.Minute).Unix() < claims.NotBefore {
			return nil, errors.New("token is not yet valid (nbf)")
		}
	}

	// 5. Validate Issuer (handles trailing slash normalization and domain variants)
	if err := v.validateIssuer(claims.Issuer); err != nil {
		return nil, err
	}

	// 6. Validate Audience claim
	if err := v.validateAudience(claims.Audience); err != nil {
		return nil, err
	}

	return &AutheliaClaims{
		Subject:           claims.Subject,
		PreferredUsername: claims.PreferredUsername,
		Name:              claims.Name,
		Email:             claims.Email,
	}, nil
}

func (v *OIDCVerifier) validateIssuer(tokenIssuer string) error {
	trimmedToken := strings.TrimRight(strings.TrimSpace(tokenIssuer), "/")
	if trimmedToken == "" {
		return errors.New("token is missing mandatory 'iss' claim")
	}

	candidates := []string{
		strings.TrimRight(v.issuer, "/"),
	}
	if v.autheliaDomain != "" {
		domain := strings.TrimRight(v.autheliaDomain, "/")
		candidates = append(candidates, "https://"+domain, "http://"+domain, domain)
	}

	for _, c := range candidates {
		if c != "" && strings.EqualFold(trimmedToken, c) {
			return nil
		}
	}

	return fmt.Errorf("token issuer %q does not match configured issuer %q", tokenIssuer, v.issuer)
}

// allowedAudiences returns the audiences this backend accepts.
//
// AUTHELIA_AUDIENCE is required and is the only accepted audience. The OIDC
// client ID is not an API audience, so ID tokens are not accepted as API tokens.
func (v *OIDCVerifier) allowedAudiences() []string {
	if strings.TrimSpace(v.audience) == "" {
		return nil
	}
	return []string{v.audience}
}

func (v *OIDCVerifier) validateAudience(rawAud json.RawMessage) error {
	tokenAudiences := extractAudiences(rawAud)
	if len(tokenAudiences) == 0 {
		return errors.New("token is missing mandatory 'aud' claim")
	}

	allowed := v.allowedAudiences()
	for _, tokenAud := range tokenAudiences {
		normTokenAud := strings.TrimRight(strings.TrimSpace(tokenAud), "/")
		for _, a := range allowed {
			if strings.EqualFold(normTokenAud, strings.TrimRight(strings.TrimSpace(a), "/")) {
				return nil
			}
		}
	}
	return fmt.Errorf("token audience %v does not match allowed audiences", tokenAudiences)
}

func extractAudiences(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if single != "" {
			return []string{single}
		}
		return nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	return nil
}

// Middleware enforces Authelia OIDC authentication and auto-provisions user records.
func Middleware(verifier *OIDCVerifier, userRepo *repository.UserRepository, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := extractToken(r)
			if rawToken == "" {
				writeAuthError(w, http.StatusUnauthorized, "missing authentication token")
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

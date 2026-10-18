package botauth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Authenticator interface {
	Authenticate(context.Context, string, string) (Identity, error)
}
type Budget interface {
	Allow(string) (bool, time.Duration)
}
type Principal struct{ ID, DisplayName string }
type Resolver func(context.Context, Identity) (Principal, error)
type principalKey struct{}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Gate is constructed only for message endpoints. HumanOnly is the existing
// bearer middleware chain, including its own budget; it never calls LDAP.
type Gate struct {
	Directory          Authenticator
	Resolve            Resolver
	Ingress, API       Budget
	HumanOnly          func(http.Handler) http.Handler
	Logger             *slog.Logger
	SyncPath, SendPath string
	TrustedProxies     []netip.Prefix
	Timeout            time.Duration
}

func (g Gate) Handler(next http.Handler) (http.Handler, error) {
	if g.Directory == nil || g.Resolve == nil || g.Ingress == nil || g.API == nil || g.HumanOnly == nil || g.Logger == nil || next == nil || g.Timeout <= 0 || g.Timeout > 30*time.Second {
		return nil, errors.New("bot message gate requires authentication, resolution, budgets, human middleware and a bounded timeout")
	}
	if (g.SyncPath == "" && g.SendPath == "") || (g.SyncPath != "" && !strings.HasPrefix(g.SyncPath, "/")) || (g.SendPath != "" && !strings.HasPrefix(g.SendPath, "/")) {
		return nil, errors.New("explicit message endpoint paths are required")
	}
	for _, prefix := range g.TrustedProxies {
		if !prefix.IsValid() {
			return nil, errors.New("invalid trusted proxy prefix")
		}
	}
	g.TrustedProxies = append([]netip.Prefix(nil), g.TrustedProxies...)
	human := g.HumanOnly(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		values := r.Header.Values("Authorization")
		if len(values) != 1 || len(values[0]) > 8192 {
			failure(w, http.StatusUnauthorized)
			return
		}
		parts := strings.Fields(values[0])
		if len(parts) != 2 || strings.Contains(values[0], ",") {
			failure(w, 401)
			return
		}
		if strings.EqualFold(parts[0], "Bearer") {
			human.ServeHTTP(w, r)
			return
		}
		if !strings.EqualFold(parts[0], "Basic") {
			failure(w, 401)
			return
		}
		// Exact method/path checks prevent accidental wrapping of the wider mux.
		if !((r.Method == http.MethodGet && r.URL.Path == g.SyncPath) || (r.Method == http.MethodPost && r.URL.Path == g.SendPath)) {
			failure(w, 401)
			return
		}
		if !g.secure(r) {
			failure(w, 403)
			return
		}
		if !allow(w, g.Ingress, "bot-auth") {
			return
		}
		username, password, ok := r.BasicAuth()
		if !ok || !validCredentials(username, password) {
			failure(w, 401)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), g.Timeout)
		defer cancel()
		identity, err := g.Directory.Authenticate(ctx, username, password)
		if err != nil {
			status, reason := 503, "directory_unavailable"
			if errors.Is(err, ErrCredentials) {
				status, reason = 401, "invalid_credentials"
			}
			if errors.Is(err, ErrForbidden) {
				status, reason = 403, "group_required"
			}
			g.Logger.Info("bot authentication rejected", "reason", reason)
			failure(w, status)
			return
		}
		principal, err := g.Resolve(ctx, identity)
		id, idErr := uuid.Parse(principal.ID)
		if err != nil || idErr != nil || id == uuid.Nil || ctx.Err() != nil {
			g.Logger.Warn("bot authentication rejected", "reason", "principal_unavailable")
			failure(w, 503)
			return
		}
		principal.ID = id.String()
		if !allow(w, g.API, "bot:"+principal.ID) {
			return
		}
		g.Logger.Info("bot authentication accepted", "principal_id", principal.ID)
		// No human UserContextKey, OIDC subject or reusable token is created.
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, principal)))
	}), nil
}

func (g Gate) secure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	headers := r.Header.Values("X-Forwarded-Proto")
	if len(headers) != 1 || headers[0] != "https" {
		return false
	}
	for _, prefix := range g.TrustedProxies {
		if prefix.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

func allow(w http.ResponseWriter, b Budget, key string) bool {
	ok, wait := b.Allow(key)
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(wait.Seconds())))))
		failure(w, 429)
	}
	return ok
}

func failure(w http.ResponseWriter, status int) {
	code, message := "unauthorized", "valid authentication required"
	switch status {
	case 403:
		code, message = "forbidden", "request not permitted"
	case 429:
		code, message = "rate_limited", "request budget exhausted"
	case 503:
		code, message = "unavailable", "authentication temporarily unavailable"
		w.Header().Set("Retry-After", "1")
	}
	if status == 401 {
		w.Header().Add("WWW-Authenticate", `Basic realm="meowgram-messages", charset="UTF-8"`)
		w.Header().Add("WWW-Authenticate", `Bearer realm="meowgram-messages"`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}

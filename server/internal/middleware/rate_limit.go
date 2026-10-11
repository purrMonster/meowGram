package middleware

import (
	"math"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/ratelimit"
	"net/http"
	"strconv"
)

func UserRateLimit(limiter *ratelimit.Group) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok || user == nil {
				http.Error(w, "Unauthorized", 401)
				return
			}
			if allowed, wait := limiter.Allow(user.AutheliaSub); !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
				http.Error(w, "Too many requests", 429)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// A process-wide ingress budget limits unauthenticated verification work. It
// deliberately ignores forwarded IP headers, which a client could spoof.
func GlobalRateLimit(limiter *ratelimit.Group, healthPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/healthz" && r.URL.Path != healthPath {
				if allowed, wait := limiter.Allow("ingress"); !allowed {
					w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
					http.Error(w, "Too many requests", 429)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

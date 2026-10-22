package botauth_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"meowgram/server/internal/auth"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/ratelimit"
)

const testID = "11111111-1111-4111-8111-111111111111"

type authFunc func(context.Context, string, string) (botauth.Identity, error)

func (f authFunc) Authenticate(c context.Context, u, p string) (botauth.Identity, error) {
	return f(c, u, p)
}

func TestMessageGateBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, path, method, header string
		duplicate, tls, trusted    bool
		forwarded                  string
		authErr                    error
		status, calls              int
	}{
		{name: "bot", path: "/sync", method: "GET", tls: true, status: 204, calls: 1},
		{name: "send", path: "/send", method: "POST", tls: true, status: 204, calls: 1},
		{name: "bearer independent", path: "/sync", method: "GET", header: "Bearer opaque", status: 202},
		{name: "failed bearer no fallback", path: "/sync", method: "GET", header: "Bearer bad", status: 401},
		{name: "multiple", path: "/sync", method: "GET", tls: true, duplicate: true, status: 401},
		{name: "combined", path: "/sync", method: "GET", tls: true, header: "Bearer one, Basic two", status: 401},
		{name: "malformed", path: "/sync", method: "GET", tls: true, header: "Basic broken", status: 401},
		{name: "ticket", path: "/api/ws-ticket", method: "POST", tls: true, status: 401},
		{name: "socket", path: "/ws", method: "GET", tls: true, status: 401},
		{name: "push", path: "/api/push/devices", method: "PUT", tls: true, status: 401},
		{name: "media", path: "/media", method: "GET", tls: true, status: 401},
		{name: "wrong method", path: "/sync", method: "POST", tls: true, status: 401},
		{name: "plaintext", path: "/sync", method: "GET", status: 403},
		{name: "spoofed proxy", path: "/sync", method: "GET", forwarded: "https", status: 403},
		{name: "trusted proxy", path: "/sync", method: "GET", forwarded: "https", trusted: true, status: 204, calls: 1},
		{name: "forwarded chain", path: "/sync", method: "GET", forwarded: "https, http", trusted: true, status: 403},
		{name: "bad password", path: "/sync", method: "GET", tls: true, authErr: botauth.ErrCredentials, status: 401, calls: 1},
		{name: "not member", path: "/sync", method: "GET", tls: true, authErr: botauth.ErrForbidden, status: 403, calls: 1},
		{name: "outage", path: "/sync", method: "GET", tls: true, authErr: errors.New("sensitive-provider-detail"), status: 503, calls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls, resolves := 0, 0
			var logs bytes.Buffer
			gate := botauth.Gate{SyncPath: "/sync", SendPath: "/send", Timeout: time.Second, Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Ingress: ratelimit.New(60, 10), API: ratelimit.New(60, 10),
				Directory: authFunc(func(context.Context, string, string) (botauth.Identity, error) {
					calls++
					return botauth.Identity{ExternalID: testID}, test.authErr
				}),
				Resolve: func(context.Context, botauth.Identity) (botauth.Principal, error) {
					resolves++
					return botauth.Principal{ID: testID}, nil
				},
				HumanOnly: func(http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Header.Get("Authorization") == "Bearer opaque" {
							w.WriteHeader(202)
						} else {
							w.WriteHeader(401)
						}
					})
				},
			}
			if test.trusted {
				gate.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("192.0.2.1/32")}
			}
			handler, err := gate.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if p, ok := botauth.PrincipalFromContext(r.Context()); !ok || p.ID != testID {
					t.Fatal("missing bot identity")
				}
				if _, ok := auth.UserFromContext(r.Context()); ok {
					t.Fatal("bot impersonates human")
				}
				w.WriteHeader(204)
			}))
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(test.method, "http://example.com"+test.path, nil)
			r.RemoteAddr = "192.0.2.1:1234"
			r.SetBasicAuth("bot", "private-test-value")
			if test.header != "" {
				r.Header.Set("Authorization", test.header)
			}
			if test.duplicate {
				r.Header.Add("Authorization", r.Header.Get("Authorization"))
			}
			if test.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if test.forwarded != "" {
				r.Header.Set("X-Forwarded-Proto", test.forwarded)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != test.status || calls != test.calls {
				t.Fatalf("status %d calls %d", w.Code, calls)
			}
			if test.authErr != nil && resolves != 0 {
				t.Fatal("unauthorized principal provisioned")
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cacheable authentication")
			}
			for _, secret := range []string{"private-test-value", "sensitive-provider-detail"} {
				if strings.Contains(logs.String()+w.Body.String(), secret) {
					t.Fatal("sensitive detail leaked")
				}
			}
		})
	}
}

func TestGateBudgetsAndResolverFailure(t *testing.T) {
	for _, stage := range []string{"ingress", "api", "resolver"} {
		t.Run(stage, func(t *testing.T) {
			calls, resolves := 0, 0
			ingress, api := ratelimit.New(1, 1), ratelimit.New(1, 1)
			if stage == "ingress" {
				ingress.Allow("bot-auth")
			}
			if stage == "api" {
				api.Allow("bot:" + testID)
			}
			g := botauth.Gate{SyncPath: "/sync", Timeout: time.Second, Logger: slog.Default(), Ingress: ingress, API: api,
				HumanOnly: func(h http.Handler) http.Handler { return h },
				Directory: authFunc(func(context.Context, string, string) (botauth.Identity, error) {
					calls++
					return botauth.Identity{}, nil
				}),
				Resolve: func(context.Context, botauth.Identity) (botauth.Principal, error) {
					resolves++
					if stage == "resolver" {
						return botauth.Principal{}, errors.New("sensitive")
					}
					return botauth.Principal{ID: testID}, nil
				},
			}
			h, err := g.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("budget bypass") }))
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "https://example.com/sync", nil)
			r.SetBasicAuth("bot", "test-value")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			want := 429
			if stage == "resolver" {
				want = 503
			}
			if w.Code != want || w.Header().Get("Retry-After") == "" {
				t.Fatal("wrong failure response")
			}
			if stage == "ingress" && (calls != 0 || resolves != 0) {
				t.Fatal("expensive work before ingress budget")
			}
		})
	}
}

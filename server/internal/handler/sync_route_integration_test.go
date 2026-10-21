package handler

import (
	"context"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/botauth"
	"meowgram/server/internal/model"
	"meowgram/server/internal/ratelimit"
	"meowgram/server/internal/repository"
	"meowgram/server/internal/testutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSyncRouteAliasesAndBasicIsolation(t *testing.T) {
	db, m, _ := testutil.Database(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := repository.NewMessageRepository(db)
	human := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer verified-test-human" {
				w.WriteHeader(401)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), auth.UserContextKey, &model.User{AutheliaSub: "human", Username: "human"})))
		})
	}
	for _, path := range []string{"/api/messages/sync", "/custom/sync"} {
		for _, enabled := range []bool{false, true} {
			name := path
			if enabled {
				name += "/basic-enabled"
			}
			t.Run(name, func(t *testing.T) {
				var gate *botauth.Gate
				if enabled {
					gate = &botauth.Gate{Directory: sendDirectoryFunc(func(context.Context, string, string) (botauth.Identity, error) {
						return botauth.Identity{Username: "bot"}, nil
					}), Resolve: func(context.Context, botauth.Identity) (botauth.Principal, error) {
						return botauth.Principal{ID: uuid.NewString(), Username: "bot"}, nil
					}, Ingress: ratelimit.New(600, 600), API: ratelimit.New(600, 600), Logger: logger, Timeout: time.Second}
				}
				mux := http.NewServeMux()
				// This is the same registration function main uses; duplicate patterns panic here.
				if err := RegisterSyncRoutes(mux, path, repo, logger, human, gate); err != nil {
					t.Fatal(err)
				}
				for _, alias := range []string{"/api/messages/sync", path} {
					for _, basic := range []bool{false, true} {
						r := httptest.NewRequest("GET", "https://example.home.arpa"+alias+"?after=2100-01-01T00:00:00Z", nil)
						r.Header.Set("Authorization", "Bearer verified-test-human")
						if basic {
							r.SetBasicAuth("bot", "synthetic")
						}
						w := httptest.NewRecorder()
						mux.ServeHTTP(w, r)
						want := 200
						if basic && !enabled {
							want = 401
						}
						if w.Code != want {
							t.Fatalf("%s basic=%v: %d want %d", alias, basic, w.Code, want)
						}
					}
					for _, method := range []string{"HEAD", "POST", "PUT", "DELETE"} {
						r := httptest.NewRequest(method, "https://example.home.arpa"+alias, nil)
						r.SetBasicAuth("bot", "synthetic")
						w := httptest.NewRecorder()
						mux.ServeHTTP(w, r)
						if w.Code < 400 {
							t.Fatalf("Basic admitted %s %s: %d", method, alias, w.Code)
						}
					}
				}
				for _, other := range []string{path + "/child", "/api/ws-ticket", "/api/push/devices", "/api/media"} {
					r := httptest.NewRequest("GET", "https://example.home.arpa"+other, nil)
					r.SetBasicAuth("bot", "synthetic")
					w := httptest.NewRecorder()
					mux.ServeHTTP(w, r)
					if w.Code < 400 {
						t.Fatalf("Basic admitted %s", other)
					}
				}
			})
		}
	}
}

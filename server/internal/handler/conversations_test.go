package handler

import (
	"context"
	"errors"
	"fmt"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/config"
	"meowgram/server/internal/middleware"
	"meowgram/server/internal/model"
	"meowgram/server/internal/repository"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type conversationStub struct {
	ConversationStore
	calls  int
	target repository.PrincipalRef
	err    error
}

func (s *conversationStub) ChangeMember(_ context.Context, actor, channel string, p repository.PrincipalRef, add bool) error {
	s.calls++
	s.target = p
	return s.err
}
func (s *conversationStub) List(context.Context, string, int, string) (repository.ConversationPage, error) {
	s.calls++
	return repository.ConversationPage{}, s.err
}
func (s *conversationStub) Create(context.Context, string, *string, []repository.PrincipalRef, bool) (repository.Conversation, bool, error) {
	s.calls++
	return repository.Conversation{}, true, s.err
}
func conversationTestMux(s *conversationStub) *http.ServeMux {
	m := http.NewServeMux()
	RegisterConversationRoutes(m, "/custom/conversations", "/custom/principals", s, func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), auth.UserContextKey, &model.User{AutheliaSub: "verified"}))
			h.ServeHTTP(w, r)
		})
	})
	return m
}
func TestConversationBasicRejectedBeforeParsing(t *testing.T) {
	for _, root := range []string{"/api/conversations", "/custom/conversations", "/api/principals", "/custom/principals"} {
		for _, suffix := range []string{"", "/start-chat", "/bad/members", "/lounge/mute", "/bad/members/human/opaque%2Fid"} {
			for _, method := range []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"} {
				t.Run(root+suffix+method, func(t *testing.T) {
					s := &conversationStub{}
					r := httptest.NewRequest(method, root+suffix+"?bad=1&bad=2", strings.NewReader("not json"))
					r.Header.Set("Authorization", "bAsIc invalid")
					w := httptest.NewRecorder()
					conversationTestMux(s).ServeHTTP(w, r)
					if w.Code != 401 || s.calls != 0 || w.Header().Get("Cache-Control") != "no-store" {
						t.Fatalf("status=%d calls=%d cache=%q", w.Code, s.calls, w.Header().Get("Cache-Control"))
					}
				})
			}
		}
	}
}
func TestConversationStrictInputs(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/conversations", `{"name":"n","name":"m","members":[]}`},
		{"POST", "/api/conversations", `{"Name":"n","members":[]}`},
		{"POST", "/api/conversations", `{"name":"n","members":[{"Kind":"human","id":"a"}]}`},
		{"POST", "/api/conversations", `{"name":"n","members":[{"kind":"human","id":"a","id":"b"}]}`},
		{"POST", "/api/conversations", `{"name":"n","members":[{"kind":"human","id":"a","other":1}]}`},
		{"POST", "/api/conversations", `{"name":"n","members":[],"other":1}`},
		{"POST", "/api/conversations/start-chat", `{"name":null,"members":[]}`},
		{"PATCH", "/api/conversations/11111111-1111-1111-1111-111111111111", `{}`},
		{"PUT", "/api/conversations/lounge/mute", `{"muted":null}`},
		{"PUT", "/api/conversations/lounge/read", `{"message_id":"null"}`},
		{"GET", "/api/conversations?limit=1&limit=2", ""},
		{"GET", "/api/conversations?scope=member", ""},
		{"GET", "/api/conversations?limit=101", ""},
		{"GET", "/api/conversations?cursor=", ""},
		{"GET", "/api/principals?q=a&q=b", ""},
		{"GET", "/api/conversations/null/members", ""},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			s := &conversationStub{}
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			conversationTestMux(s).ServeHTTP(w, r)
			if w.Code != 400 || s.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, s.calls, w.Body.String())
			}
		})
	}
}
func TestConversationOpaquePathAndSafeErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{nil, 204}, {repository.ErrConversationForbidden, 403}, {repository.ErrConversationBusy, 503}, {errors.New("private database detail"), 503}} {
		s := &conversationStub{err: tc.err}
		r := httptest.NewRequest("DELETE", "/custom/conversations/11111111-1111-1111-1111-111111111111/members/human/issuer%2Fsubject%25value", nil)
		w := httptest.NewRecorder()
		conversationTestMux(s).ServeHTTP(w, r)
		if w.Code != tc.status || s.target.ID != "issuer/subject%value" || s.target.Kind != "human" {
			t.Fatalf("status=%d target=%+v", w.Code, s.target)
		}
		if strings.Contains(w.Body.String(), "private") {
			t.Fatal("leaked error")
		}
		if tc.status == 503 && w.Header().Get("Retry-After") == "" {
			t.Fatal("missing retry hint")
		}
		if tc.status == 403 && w.Body.String() != "{\"error\":\"forbidden\",\"message\":\"request not permitted\"}\n" {
			t.Fatal(w.Body.String())
		}
	}
}
func TestConversationAliasesRetainDefaults(t *testing.T) {
	for _, root := range []string{"/api/conversations", "/custom/conversations"} {
		s := &conversationStub{}
		w := httptest.NewRecorder()
		conversationTestMux(s).ServeHTTP(w, httptest.NewRequest("GET", root, nil))
		if w.Code != 200 || s.calls != 1 {
			t.Fatalf("status=%d calls=%d", w.Code, s.calls)
		}
	}
}

func TestConversationBasicGuardBeforeCORS(t *testing.T) {
	const origin = "https://client.example.home.arpa"
	for _, path := range []string{"/api/conversations", "/api/conversations/start-chat", "/api/principals", "/custom/conversations", "/custom/conversations/group/members", "/custom/principals"} {
		for _, basic := range []bool{true, false} {
			t.Run(path+fmt.Sprint(basic), func(t *testing.T) {
				called := false
				next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(202) })
				handler := ConversationBasicGuard(middleware.CORS(&config.Config{CORSOrigins: []string{origin}})(next), "/custom/conversations", "/custom/principals")
				req := httptest.NewRequest("OPTIONS", path, nil)
				req.Header.Set("Origin", origin)
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
				if basic {
					req.Header.Set("Authorization", "bAsIc invalid")
				}
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				want := 204
				if basic {
					want = 401
				}
				if w.Code != want || called {
					t.Fatalf("status=%d next=%v", w.Code, called)
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("missing no-store")
				}
				if !basic && w.Header().Get("Access-Control-Allow-Origin") != origin {
					t.Fatal("genuine browser preflight changed")
				}
			})
		}
	}
	for _, path := range []string{"/api/messages", "/api/conversations-other", "/custom/principals-extra"} {
		for _, method := range []string{"OPTIONS", "GET"} {
			t.Run(path+method, func(t *testing.T) {
				next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202) })
				h := ConversationBasicGuard(middleware.CORS(&config.Config{})(next), "/custom/conversations", "/custom/principals")
				req := httptest.NewRequest(method, path, nil)
				req.SetBasicAuth("fixture", "fixture")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, req)
				want := 202
				if method == "OPTIONS" {
					want = 204
				}
				if w.Code != want || w.Header().Get("Cache-Control") != "" {
					t.Fatalf("unrelated route changed: %d", w.Code)
				}
			})
		}
	}
}

func TestConversationUnicodeAliasEscapedPath(t *testing.T) {
	store := &conversationStub{}
	mux := http.NewServeMux()
	RegisterConversationRoutes(mux, "/api/会話", "/api/人", store, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), auth.UserContextKey, &model.User{AutheliaSub: "verified"}))
			next.ServeHTTP(w, r)
		})
	})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/%E4%BC%9A%E8%A9%B1", nil))
	if w.Code != 200 || store.calls != 1 {
		t.Fatalf("unicode list status=%d calls=%d", w.Code, store.calls)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/%E4%BC%9A%E8%A9%B1/11111111-1111-1111-1111-111111111111/members/human/issuer%2Fsubject%25value", nil))
	if w.Code != 204 || store.target.ID != "issuer/subject%value" {
		t.Fatalf("unicode member status=%d target=%+v", w.Code, store.target)
	}
}

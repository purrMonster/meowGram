package handler

import (
	"context"
	"io"
	"log/slog"
	"meowgram/server/internal/auth"
	"meowgram/server/internal/model"
	"meowgram/server/internal/repository"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScopedSyncRejectsAmbiguousSelectorsBeforeStorage(t *testing.T) {
	for _, query := range []string{
		"after=0&after=1", "after=0&after_id=&after_id=x", "after=0&channel_id=11111111-1111-1111-1111-111111111111&scope=member",
		"after=0&channel_id=null", "after=0&channel_id=", "after=0&channel_id=bad", "after=0&scope=member&scope=member", "after=0&scope=all", "after=0&scope=", "after=0&unknown=x", "after=0&channel_id=%zz",
	} {
		t.Run(query, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/messages/sync?"+query, nil)
			r = r.WithContext(context.WithValue(r.Context(), auth.UserContextKey, &model.User{AutheliaSub: "fixture"}))
			w := httptest.NewRecorder()
			SyncHandler(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))(w, r)
			if w.Code != 400 {
				t.Fatalf("status=%d", w.Code)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cacheable error")
			}
		})
	}
}
func TestScopedSendRejectsAmbiguousBodyBeforeStorage(t *testing.T) {
	for _, body := range []string{
		`{"text_content":"first","text_content":"second"}`, `{"text_content":"x","channel_id":null,"channel_id":null}`, `{"text_content":"x","channel_id":""}`, `{"text_content":"x","channel_id":"null"}`, `{"text_content":"x","channel_id":3}`, `{"text_content":"x","Channel_id":null}`, `{"text_content":"x","scope":"member"}`,
	} {
		t.Run(body, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/messages", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Idempotency-Key", "fixture")
			r = r.WithContext(context.WithValue(r.Context(), auth.UserContextKey, &model.User{AutheliaSub: "fixture"}))
			w := httptest.NewRecorder()
			SendHandler(nil, time.Hour).ServeHTTP(w, r)
			if w.Code != 400 {
				t.Fatalf("status=%d", w.Code)
			}
		})
	}
}

type ticketConversationStub struct {
	ConversationStore
	calls int
	err   error
}

func (s *ticketConversationStub) Members(context.Context, string, string, int, string) (repository.MemberPage, error) {
	s.calls++
	return repository.MemberPage{}, s.err
}
func TestTicketSelectorValidationAndAccess(t *testing.T) {
	for _, tc := range []struct {
		body          string
		err           error
		status, calls int
	}{
		{"", nil, 200, 0}, {`{}`, nil, 200, 0}, {`{"channel_id":null}`, nil, 200, 0},
		{`{"channel_id":"11111111-1111-1111-1111-111111111111"}`, nil, 200, 1},
		{`{"channel_id":"11111111-1111-1111-1111-111111111111"}`, repository.ErrConversationForbidden, 403, 1},
		{`{"channel_id":"null"}`, nil, 400, 0}, {`{"channel_id":""}`, nil, 400, 0},
		{`{"channel_id":null,"channel_id":null}`, nil, 400, 0}, {`{"scope":"member"}`, nil, 400, 0},
	} {
		t.Run(tc.body+string(rune(tc.status)), func(t *testing.T) {
			stub := &ticketConversationStub{err: tc.err}
			r := httptest.NewRequest("POST", "/api/ws-ticket", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r = r.WithContext(context.WithValue(r.Context(), auth.UserContextKey, &model.User{AutheliaSub: "fixture"}))
			w := httptest.NewRecorder()
			WSTicketHandler(auth.NewWSTicketStore(), slog.New(slog.NewTextHandler(io.Discard, nil)), stub)(w, r)
			if w.Code != tc.status || stub.calls != tc.calls {
				t.Fatalf("status=%d calls=%d", w.Code, stub.calls)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cacheable ticket result")
			}
		})
	}
}

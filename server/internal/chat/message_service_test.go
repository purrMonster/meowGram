package chat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"meowgram/server/internal/model"
	"meowgram/server/internal/ratelimit"
)

type storeFunc func(context.Context, string, string, string) (*model.Message, error)

func (f storeFunc) Create(c context.Context, id, name, text string) (*model.Message, error) {
	return f(c, id, name, text)
}

type publishFunc func(*model.WSMessage) bool

func TestMessageServicePublishesStoredSenderKind(t *testing.T) {
	for _, kind := range []string{"bot", "human"} {
		t.Run(kind, func(t *testing.T) {
			stored := &model.Message{ID: "persisted", SenderID: "sender", SenderKind: kind, TextContent: "hello"}
			published := 0
			service := NewMessageService(storeFunc(func(context.Context, string, string, string) (*model.Message, error) {
				return stored, nil
			}), publishFunc(func(message *model.WSMessage) bool {
				published++
				if message.SenderKind != kind || message.SenderID != stored.SenderID || message.ID != stored.ID {
					t.Fatalf("publication lost stored identity: %+v", message)
				}
				return true
			}), nil)
			result, err := service.Send(context.Background(), "sender", "name", "hello")
			if err != nil || result != stored || published != 1 {
				t.Fatalf("send result=%v err=%v publications=%d", result, err, published)
			}
		})
	}
}

func (f publishFunc) Publish(m *model.WSMessage) bool { return f(m) }

func TestMessageServiceCommitBeforePublication(t *testing.T) {
	committed := false
	stored := &model.Message{ID: "stored-id", SenderID: "subject", Username: "name", TextContent: "🐾", CreatedAt: time.Now().UTC()}
	s := NewMessageService(storeFunc(func(ctx context.Context, id, name, text string) (*model.Message, error) {
		if id != "subject" || name != "name" || text != "🐾" {
			t.Fatal("sender or trimmed Unicode text changed")
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("missing bounded write deadline")
		}
		committed = true
		return stored, nil
	}), publishFunc(func(m *model.WSMessage) bool {
		if !committed {
			t.Fatal("publication before commit")
		}
		if m.Type != "chat" || m.ID != stored.ID || m.SenderID != stored.SenderID || m.Username != stored.Username || m.TextContent != stored.TextContent || m.CreatedAt != stored.CreatedAt {
			t.Fatal("publication differs from committed message")
		}
		return true
	}), nil)
	result, err := s.Send(context.Background(), "subject", "name", " \t🐾\n")
	if err != nil || result != stored {
		t.Fatalf("send failed: %v", err)
	}
}

func TestMessageServiceRejectsWithoutSideEffects(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       error
	}{
		{"blank", " \n\t", ErrEmptyMessage},
		{"Unicode over limit", strings.Repeat("🐾", 4001), ErrMessageTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewMessageService(nil, nil, nil)
			m, err := s.Send(context.Background(), "subject", "name", tc.text)
			if m != nil || !errors.Is(err, tc.want) {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
}

func TestMessageServiceWriteFailureAndCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		}
		cause := errors.New("database unavailable")
		s := NewMessageService(storeFunc(func(ctx context.Context, _, _, _ string) (*model.Message, error) {
			if cancelled {
				return nil, ctx.Err()
			}
			return nil, cause
		}), publishFunc(func(*model.WSMessage) bool { t.Fatal("published a failed write"); return false }), nil)
		m, err := s.Send(ctx, "subject", "name", "hello")
		cancel()
		if cancelled {
			cause = context.Canceled
		}
		if m != nil || !errors.Is(err, ErrMessagePersistence) || !errors.Is(err, cause) {
			t.Fatalf("failure lost: %v", err)
		}
	}
}

func TestMessageServiceStoppedHubRetainsCommittedResult(t *testing.T) {
	stored := &model.Message{ID: "durable"}
	s := NewMessageService(storeFunc(func(context.Context, string, string, string) (*model.Message, error) { return stored, nil }),
		publishFunc(func(*model.WSMessage) bool { return false }), nil)
	m, err := s.Send(context.Background(), "subject", "name", strings.Repeat("🐾", 4000))
	if m != stored || !errors.Is(err, ErrBroadcastStopped) {
		t.Fatalf("lost durable result: %v", err)
	}
}

func TestMessageServiceSharedBudgetAcrossAdapters(t *testing.T) {
	// Separate adapters/reconnections using the same hub must not reset budget.
	hub := NewHub(quietLogger(), nil, ratelimit.New(1, 1))
	var writes atomic.Int32
	store := storeFunc(func(_ context.Context, id, name, text string) (*model.Message, error) {
		writes.Add(1)
		return &model.Message{SenderID: id, Username: name, TextContent: text}, nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := hub.MessageService(store).Send(context.Background(), "shared", "name", "hello")
			if err != nil {
				var limited *MessageRateError
				if !errors.As(err, &limited) || limited.RetryAfter <= 0 {
					t.Errorf("unexpected error: %v", err)
				}
			}
		}()
	}
	wg.Wait()
	if writes.Load() != 1 || len(hub.Broadcast) != 1 {
		t.Fatal("shared budget allowed duplicate writes")
	}
	if _, err := hub.MessageService(store).Send(context.Background(), "other", "name", "hello"); err != nil {
		t.Fatal("other principal was throttled")
	}
}

func TestMessageServiceInvalidAttemptConsumesBudget(t *testing.T) {
	s := NewMessageService(nil, nil, ratelimit.New(1, 1))
	if _, err := s.Send(context.Background(), "subject", "", ""); !errors.Is(err, ErrEmptyMessage) {
		t.Fatal(err)
	}
	if _, err := s.Send(context.Background(), "subject", "", "valid"); !errors.Is(err, ErrMessageRateLimited) {
		t.Fatal("invalid attempt did not consume budget")
	}
}

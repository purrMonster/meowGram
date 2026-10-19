package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"meowgram/server/internal/model"
)

// MaxTextRunes is shared by all message transports.
const MaxTextRunes = 4000

var (
	ErrEmptyMessage       = errors.New("empty message")
	ErrMessageTooLong     = errors.New("message exceeds 4000 characters")
	ErrMessageRateLimited = errors.New("message rate limit reached")
	ErrMessagePersistence = errors.New("message persistence failed")
	ErrBroadcastStopped   = errors.New("message committed but broadcast hub stopped")
)

// MessageStore returns only after the message has committed.
type MessageStore interface {
	Create(context.Context, string, string, string) (*model.Message, error)
}

type MessagePublisher interface {
	Publish(*model.WSMessage) bool
}

type MessageBudget interface {
	Allow(string) (bool, time.Duration)
}

// MessageRateError exposes the delay for future HTTP Retry-After responses.
type MessageRateError struct{ RetryAfter time.Duration }

func (e *MessageRateError) Error() string { return ErrMessageRateLimited.Error() }
func (e *MessageRateError) Unwrap() error { return ErrMessageRateLimited }

// MessageService owns validation, the shared principal budget, commit and
// publication. The Hub retains fan-out and coalesced, content-free push delivery.
// All transports must use the same budget instance, including across reconnects.
type MessageService struct {
	store     MessageStore
	publisher MessagePublisher
	budget    MessageBudget
}

func NewMessageService(store MessageStore, publisher MessagePublisher, budget MessageBudget) *MessageService {
	return &MessageService{store: store, publisher: publisher, budget: budget}
}

// Send accepts text and an already authenticated sender. It does not authenticate.
// Attempts consume budget before validation, retaining the WebSocket policy.
// A non-nil message with ErrBroadcastStopped is durable; callers must not treat
// it as a failed insert. Publication does not acknowledge delivery to clients.
func (s *MessageService) Send(ctx context.Context, senderID, username, text string) (*model.Message, error) {
	if s.budget != nil {
		if allowed, retry := s.budget.Allow(senderID); !allowed {
			return nil, &MessageRateError{RetryAfter: retry}
		}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrEmptyMessage
	}
	if utf8.RuneCountInString(text) > MaxTextRunes {
		return nil, ErrMessageTooLong
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	message, err := s.store.Create(writeCtx, senderID, username, text)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMessagePersistence, err)
	}
	if !s.publisher.Publish(&model.WSMessage{
		Type: "chat", ID: message.ID, SenderID: message.SenderID,
		Username: message.Username, TextContent: message.TextContent, CreatedAt: message.CreatedAt,
	}) {
		return message, ErrBroadcastStopped
	}
	return message, nil
}

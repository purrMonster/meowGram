package chat

import (
	"context"
	"errors"
	"meowgram/server/internal/model"
	"meowgram/server/internal/repository"
	"strings"
	"time"
	"unicode/utf8"
)

type IdempotentMessageStore interface {
	CreateIdempotent(context.Context, repository.SendPrincipal, string, string, string, time.Duration, func() error) (*repository.SendResult, error)
}

type ScopedIdempotentMessageStore interface {
	CreateIdempotentChannel(context.Context, repository.SendPrincipal, string, string, string, *string, time.Duration, func() error) (*repository.SendResult, error)
}

// SendIdempotent shares the WebSocket validator, write budget and publisher.
// A replay consumes the route's authentication/API budgets but no write budget.
func (s *MessageService) SendIdempotent(ctx context.Context, p repository.SendPrincipal, key, raw string, retention time.Duration) (*repository.SendResult, error) {
	return s.sendIdempotent(ctx, p, key, raw, nil, false, retention)
}

func (s *MessageService) SendIdempotentChannel(ctx context.Context, p repository.SendPrincipal, key, raw string, channel *string, retention time.Duration) (*repository.SendResult, error) {
	return s.sendIdempotent(ctx, p, key, raw, channel, true, retention)
}

func (s *MessageService) sendIdempotent(ctx context.Context, p repository.SendPrincipal, key, raw string, channel *string, scoped bool, retention time.Duration) (*repository.SendResult, error) {
	text := strings.TrimSpace(raw)
	store, ok := s.store.(IdempotentMessageStore)
	if !ok {
		return nil, ErrMessagePersistence
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	authorizeWrite := func() error {
		budgetKey := p.Kind + ":" + p.ID
		if s.budget != nil {
			if allowed, wait := s.budget.Allow(budgetKey); !allowed {
				return &MessageRateError{RetryAfter: wait}
			}
		}
		if text == "" {
			return ErrEmptyMessage
		}
		if !utf8.ValidString(raw) || utf8.RuneCountInString(text) > MaxTextRunes {
			return ErrMessageTooLong
		}
		return nil
	}
	var result *repository.SendResult
	var err error
	if scoped {
		channelStore, ok := s.store.(ScopedIdempotentMessageStore)
		if !ok {
			return nil, ErrMessagePersistence
		}
		result, err = channelStore.CreateIdempotentChannel(ctx, p, key, raw, text, channel, retention, authorizeWrite)
	} else {
		result, err = store.CreateIdempotent(ctx, p, key, raw, text, retention, authorizeWrite)
	}
	if err != nil {
		return nil, err
	}
	if !result.Replay && !s.publisher.Publish(&model.WSMessage{Type: "chat", ID: result.Message.ID, SenderID: result.Message.SenderID, SenderKind: result.Message.SenderKind, Username: result.Message.Username, TextContent: result.Message.TextContent, CreatedAt: result.Message.CreatedAt, ChannelID: result.Message.ChannelID}) {
		// The durable response is still success; sync repairs delivery after a crash
		// or stopped hub. Retrying must never create another message or publication.
		return result, ErrBroadcastStopped
	}
	return result, nil
}

func IsDurableSend(err error) bool { return errors.Is(err, ErrBroadcastStopped) }

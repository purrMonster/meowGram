package chat

import (
	"context"
	"errors"
	"meowgram/server/internal/model"
	"testing"
)

type receiptStore struct {
	replay    bool
	committed bool
}

func (s *receiptStore) Create(context.Context, string, string, string) (*model.Message, error) {
	panic("wrong persistence path")
}
func (s *receiptStore) CreateOnce(context.Context, string, string, string, string, string) (*model.Message, bool, error) {
	s.committed = true
	return &model.Message{ID: "durable", SenderKind: "bot"}, s.replay, nil
}
func TestReceiptSurvivesPublicationFailureAndReplayDoesNotPublish(t *testing.T) {
	store := &receiptStore{}
	published := 0
	service := NewMessageService(store, publishFunc(func(m *model.WSMessage) bool {
		if !store.committed || m.SenderKind != "bot" {
			t.Fatal("publication before commit or lost identity")
		}
		published++
		return false
	}), nil)
	msg, replay, err := service.SendOnce(context.Background(), "bot", "principal", "bot", "key", "text")
	if msg == nil || replay || !errors.Is(err, ErrBroadcastStopped) {
		t.Fatal("committed result lost")
	}
	store.replay = true
	msg, replay, err = service.SendOnce(context.Background(), "bot", "principal", "bot", "key", "text")
	if msg == nil || !replay || err != nil || published != 1 {
		t.Fatal("replay rebroadcast or lost receipt")
	}
}

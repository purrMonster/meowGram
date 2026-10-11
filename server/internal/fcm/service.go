package fcm

import (
	"context"
	"errors"
	"log/slog"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
	"meowgram/server/internal/repository"
)

type DeviceStore interface {
	ActiveAfter(context.Context, int64) ([]repository.PushDevice, error)
	DeleteInvalid(context.Context, string) error
}
type Sender interface {
	SendEachForMulticast(context.Context, *messaging.MulticastMessage) (*messaging.BatchResponse, error)
}
type Service struct {
	client  Sender
	devices DeviceStore
}

func NewService(ctx context.Context, logger *slog.Logger, credentialsFile string, devices DeviceStore) (*Service, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, errors.New("unable to initialize push service")
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, errors.New("unable to initialize push sender")
	}
	return &Service{client: client, devices: devices}, nil
}

// PublishActivity sends only to authenticated, unexpired device registrations.
// No public topic is used. Payloads remain content-free and collapse on devices.
func (s *Service) PublishActivity(ctx context.Context, title, body string, data map[string]string) error {
	var cursor int64
	for {
		devices, err := s.devices.ActiveAfter(ctx, cursor)
		if err != nil {
			return errors.New("unable to load push recipients")
		}
		if len(devices) == 0 {
			return nil
		}
		tokens := make([]string, len(devices))
		for i, d := range devices {
			tokens[i] = d.Token
		}
		batch, err := s.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{
			Tokens: tokens, Notification: &messaging.Notification{Title: title, Body: body}, Data: data,
			Android: &messaging.AndroidConfig{CollapseKey: "lounge_activity", Notification: &messaging.AndroidNotification{Tag: "lounge_activity"}},
			APNS:    &messaging.APNSConfig{Headers: map[string]string{"apns-collapse-id": "lounge_activity"}, Payload: &messaging.APNSPayload{Aps: &messaging.Aps{Sound: "default", ThreadID: "lounge_activity"}}},
		})
		if err != nil {
			return errors.New("push provider request failed")
		}
		failed := false
		for i, response := range batch.Responses {
			if response.Success {
				continue
			}
			if messaging.IsUnregistered(response.Error) {
				if err := s.devices.DeleteInvalid(ctx, tokens[i]); err != nil {
					return errors.New("unable to remove expired push token")
				}
			} else {
				failed = true
			}
		}
		if failed {
			return errors.New("push provider rejected one or more deliveries")
		}
		cursor = devices[len(devices)-1].ID
		if len(devices) < 500 {
			return nil
		}
	}
}

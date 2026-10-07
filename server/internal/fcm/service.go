package fcm

import (
	"context"
	"fmt"
	"log/slog"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type Service struct {
	client *messaging.Client
	logger *slog.Logger
}

func NewService(ctx context.Context, logger *slog.Logger, credentialsFile string) (*Service, error) {
	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}

	app, err := firebase.NewApp(ctx, nil, opts...)
	if err != nil {
		return nil, fmt.Errorf("error initializing firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing firebase messaging client: %w", err)
	}

	return &Service{
		client: client,
		logger: logger,
	}, nil
}

func (s *Service) PublishToTopic(ctx context.Context, topic, title, body string, data map[string]string) error {
	msg := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:  data,
		Topic: topic,
	}

	id, err := s.client.Send(ctx, msg)
	if err != nil {
		return fmt.Errorf("error sending message to topic %s: %w", topic, err)
	}

	s.logger.Debug("Successfully broadcasted push notification", "topic", topic, "fcm_message_id", id)
	return nil
}

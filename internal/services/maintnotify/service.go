package maintnotify

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/maintnotify/render"
)

type MessageSender interface {
	Send(ctx context.Context, trName entity.NotifyTransport, target string, msg entity.NotifyMessage) error
	SendAsync(ctx context.Context, taskType string, trName entity.NotifyTransport, target string, msg entity.NotifyMessage, idempotencyKey string) error
}

type NotifyTargetsStore interface {
	ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.NotifyTarget, error)
}

// Service turns maintenance-lifecycle events into Sender.SendAsync calls
type Service struct {
	frontendURL   string
	notifyTargets NotifyTargetsStore
	renderer      *render.Service
	sender        MessageSender
}

func NewNotifier(
	cfg *config.AppConfig,
	sender MessageSender,
	notifyTargets NotifyTargetsStore,
) (*Service, error) {
	rend, err := render.New()
	if err != nil {
		return nil, fmt.Errorf("init renderer: %w", err)
	}

	return &Service{
		frontendURL:   cfg.App.FrontendURL,
		notifyTargets: notifyTargets,
		renderer:      rend,
		sender:        sender,
	}, nil
}

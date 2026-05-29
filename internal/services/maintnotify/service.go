package maintnotify

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/maintnotify/render"
	messagesender "github.com/ruko1202/maintmode/internal/services/messaging/sender"
)

type NotifyTargetsStore interface {
	ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.NotifyTarget, error)
}

// Service turns maintenance-lifecycle events into Sender.SendAsync calls
type Service struct {
	frontendURL   string
	notifyTargets NotifyTargetsStore
	renderer      *render.Service
	sender        messagesender.Sender
}

func NewNotifier(
	cfg *config.AppConfig,
	sender messagesender.Sender,
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

package messagesender

import (
	"context"

	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
)

type Sender interface {
	Send(ctx context.Context, trName entity.NotifyTransport, target string, msg entity.NotifyMessage) error
	SendAsync(ctx context.Context, trName entity.NotifyTransport, target string, msg entity.NotifyMessage, opts ...EnqueueOption) error
}

// Service is the messaging facade.
type Service struct {
	notifyTransportRegistry *notifytransport.Registry
	queue                   goque.TaskQueueManager
}

func NewService(
	notifyTransportRegistry *notifytransport.Registry,
	queue goque.TaskQueueManager,
) *Service {
	return &Service{
		notifyTransportRegistry: notifyTransportRegistry,
		queue:                   queue,
	}
}

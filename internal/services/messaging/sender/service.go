package messagesender

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
)

type Sender interface {
	Send(ctx context.Context, trName entity.NotifyTransport, target string, msg entity.NotifyMessage) error
	SendAsync(ctx context.Context, trName entity.NotifyTransport, target string, msg entity.NotifyMessage, idempotencyKey string) error
}

// taskScheduler is the slice of messaging/scheduler this facade needs: enqueue a
// delivery task. messagesender builds on the scheduler instead of touching the
// queue directly, so the goque plumbing (task build + outbox + add) lives in one
// place.
type taskScheduler interface {
	ScheduleDelayed(ctx context.Context, taskType string, payload any, delay time.Duration, idempotencyKey string) (uuid.UUID, error)
}

// Service is the messaging delivery facade: synchronous Send via the transport
// registry, and SendAsync which enqueues a messaging.send task via the scheduler.
type Service struct {
	notifyTransportRegistry *notifytransport.Registry
	scheduler               taskScheduler
}

func NewService(
	notifyTransportRegistry *notifytransport.Registry,
	sched taskScheduler,
) *Service {
	return &Service{
		notifyTransportRegistry: notifyTransportRegistry,
		scheduler:               sched,
	}
}

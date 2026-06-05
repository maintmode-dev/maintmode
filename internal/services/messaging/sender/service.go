package messagesender

import (
	"context"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/gateways/notifytransport"
)

// taskScheduler is the slice of messaging/scheduler this facade needs: enqueue a
// delivery task. messagesender builds on the scheduler instead of touching the
// queue directly, so the goque plumbing (task build + outbox + add) lives in one
// place.
type taskScheduler interface {
	Schedule(ctx context.Context, taskType string, payload any, idempotencyKey string) (uuid.UUID, error)
}

// Service is the messaging delivery facade: synchronous Send via the transport
// registry, and SendAsync which enqueues a delivery task (of the caller's task
// type) via the scheduler.
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

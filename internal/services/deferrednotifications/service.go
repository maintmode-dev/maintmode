package deferrednotifications

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/storages/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Scheduler enqueues and cancels the goque tasks that drive reminders. It is the
// slice of messaging/scheduler this service needs.
type Scheduler interface {
	ScheduleDelayed(
		ctx context.Context,
		taskType string,
		payload any,
		delay time.Duration,
		idempotencyKey string,
	) (uuid.UUID, error)
	Cancel(ctx context.Context, taskID uuid.UUID) error
}

// Service orchestrates a maintenance's deferred reminders: it persists the
// schedule (CreateDraft), and on approve enqueues one goque task per reminder
// (recording its task id) so a worker can resolve the maintenance's notify
// targets and render the reminder at fire time. On cancel it cancels the
// pending tasks.
type Service struct {
	txManager                  *dbtx.TxManager
	deferredNotificationsStore *deferrednotifications.Store
	sched                      Scheduler
}

func NewService(
	txManager *dbtx.TxManager,
	store *deferrednotifications.Store,
	sched Scheduler,
) *Service {
	return &Service{
		txManager:                  txManager,
		deferredNotificationsStore: store,
		sched:                      sched,
	}
}

package deferrednotifications

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Schedule schedules one delayed goque task per reminder of the maintenance
// (delay = fire_at - now, clamped to 0 so a past fire_at fires immediately) and
// records each task id. The task carries only the maintenance/reminder ids; the
// processor resolves notify targets and renders the text at fire time.
//
// Schedule MUST run inside the caller's tx (it is called from the ApproveMaint
// transaction): the scheduler joins that tx via the transactional outbox, so the
// queued tasks, their persisted ids, and the maintenance's transition to
// "planned" all commit together — or none do. This makes approval atomic: a
// crash can't leave a scheduled maintenance whose reminders were never enqueued,
// and the recorded task ids are never out of sync with the queue. An error is
// propagated so the caller's tx rolls back.
func (s *Service) Schedule(ctx context.Context, maintID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.DeferredNotifications.Enqueue",
		xfield.String("maintID", maintID.String()),
	)
	defer span.End()

	notifications, err := s.deferredNotificationsStore.ListByMaint(ctx, maintID)
	if err != nil {
		return fmt.Errorf("list deferred notifications: %w", err)
	}

	now := xtime.UTCNow()
	for _, notification := range notifications {
		delay := max(notification.FireAt.Sub(now), 0)

		taskID, err := s.sched.ScheduleDelayed(ctx,
			entity.ProcessorTaskMaintReminder,
			entity.ProcessorTaskPayloadMaintReminder{MaintID: maintID, DeferredID: notification.ID},
			delay,
			idempotencyKey(maintID, notification.ID),
		)
		if err != nil {
			return fmt.Errorf("schedule reminder %s: %w", notification.ID, err)
		}

		if err := s.deferredNotificationsStore.SetTaskID(ctx, notification.ID, taskID); err != nil {
			return fmt.Errorf("record reminder task id %s: %w", notification.ID, err)
		}
	}

	return nil
}

// idempotencyKey makes goque's unique (type, external_id) index collapse a retry
// of the same (maint, reminder) enqueue, so it can't create a duplicate task.
func idempotencyKey(maintID, deferredID uuid.UUID) string {
	return xhash.HashSha256(fmt.Appendf(nil, "maint|reminder|%s|%s", maintID, deferredID))
}

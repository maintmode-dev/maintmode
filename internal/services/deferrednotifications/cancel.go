package deferrednotifications

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"
)

// Cancel best-effort cancels every pending reminder of a maintenance. It is a
// secondary guard: the primary one is the processor, which skips a reminder
// whose maintenance is no longer scheduled (see reminderprocessor.shouldRemind).
// So even if this cancel is lost (it is best-effort, called post-commit), a
// canceled maintenance still won't blast reminders — the task fires but no-ops.
// Canceling here just keeps the queue clean.
func (s *Service) Cancel(ctx context.Context, maintID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.DeferredNotifications.Cancel",
		xfield.String("maintID", maintID.String()),
	)
	defer span.End()

	err := s.cancelEnqueued(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "failed to cancel deferred notifications", xfield.Error(err))
		return err
	}

	return nil
}

// cancelEnqueued best-effort cancels every goque task recorded for a
// maintenance's reminders. Reminders not yet enqueued (task id NULL) are skipped
// by the deferredNotificationsStore; already-terminal tasks are no-ops in goque. Failures are logged,
// never propagated — cancellation must not fail the caller's operation.
func (s *Service) cancelEnqueued(ctx context.Context, maintID uuid.UUID) error {
	notifications, err := s.deferredNotificationsStore.ListByMaint(ctx, maintID)
	if err != nil {
		return fmt.Errorf("list deferred notifications: %w", err)
	}

	for _, notification := range notifications {
		if !notification.IsScheduled() {
			continue
		}
		taskID := lo.FromPtr(notification.GoqueTaskID)

		if err := s.sched.Cancel(ctx, taskID); err != nil {
			return fmt.Errorf("cancel task %s: %w", taskID.String(), err)
		}
	}

	return nil
}

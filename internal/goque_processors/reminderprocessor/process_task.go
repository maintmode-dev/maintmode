package reminderprocessor

import (
	"context"
	"errors"
	"fmt"

	"github.com/ruko1202/goque"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (p *processor) ProcessTask(ctx context.Context, task *goque.TypedTask[entity.ProcessorTaskPayloadMaintReminder]) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Messaging.ReminderProcessor.ProcessTask",
		xfield.String("maintID", task.Payload.MaintID.String()),
		xfield.String("deferredID", task.Payload.DeferredID.String()),
	)
	defer span.End()

	maint, err := p.maintLoader.GetMaint(ctx, task.Payload.MaintID)
	if err != nil {
		// The maintenance was deleted (cascade removed its reminders too) —
		// nothing to send; don't retry.
		if errors.Is(err, apperr.ErrMaintNotFound) {
			xlog.Warn(ctx, "reminder skipped: maintenance not found")
			return nil
		}
		return fmt.Errorf("reminder processor: load maint: %w", err)
	}

	// Only fire reminders while the maintenance is still planned. Once it has
	// started, completed, or been canceled, an "upcoming maintenance" reminder
	// is moot — and pending reminders are normally canceled on transition, but
	// this guards the race where the task fires first.
	if !shouldRemind(ctx, maint) {
		return nil
	}

	return p.notifier.NotifyMaintReminder(ctx, maint)
}

// shouldRemind reports whether a reminder should still fire for the given status.
// Only "planned" qualifies: a reminder announces an upcoming maintenance, which
// is pointless once it has already started (in_progress) or ended
// (completed/canceled).
func shouldRemind(ctx context.Context, maint *entity.Maintenance) bool {
	if maint.Status != entity.MaintenanceStatusPlanned {
		xlog.Info(ctx, "reminder skipped: maintenance is not planned",
			xfield.String("status", string(maint.Status)))
		return false
	}

	return true
}

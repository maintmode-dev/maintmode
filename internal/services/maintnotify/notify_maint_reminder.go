package maintnotify

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// NotifyMaintReminder renders the maint.reminder template and delivers it to the
// maintenance's current notify targets. Unlike the lifecycle notifications, this
// is driven by a scheduled goque task (a deferred reminder), so it returns an
// error: the worker uses it to decide whether to retry the task.
func (n *Service) NotifyMaintReminder(ctx context.Context, maint *entity.Maintenance) error {
	ctx, span := xlog.WithOperationSpan(ctx, "service.MaintNotify.NotifyMaintReminder",
		xfield.String("maintID", maint.ID.String()),
	)
	defer span.End()

	return n.dispatchSync(ctx, entity.NotifyEvent{
		Kind:            entity.NotifyEventMaintReminder,
		MaintID:         maint.ID,
		MaintTitle:      maint.Title,
		PlannedStart:    maint.PlannedPeriod.Start,
		CreatedByUserID: maint.CreatedByUserID,
	})
}

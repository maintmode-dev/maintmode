package deferrednotifications

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// SetTaskID records the goque task id enqueued for a reminder so the pending
// task can later be canceled.
func (s *Store) SetTaskID(ctx context.Context, deferredNotificationID, taskID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.DeferredNotifications.SetTaskID")
	defer span.End()

	stmt := table.MaintenanceDeferredNotifications.
		UPDATE(table.MaintenanceDeferredNotifications.GoqueTaskID).
		SET(postgres.UUID(taskID)).
		WHERE(table.MaintenanceDeferredNotifications.ID.EQ(postgres.UUID(deferredNotificationID)))

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}

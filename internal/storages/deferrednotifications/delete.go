package deferrednotifications

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// DeleteByMaint removes all deferred notifications of a maintenance.
func (s *Store) DeleteByMaint(ctx context.Context, maintID uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.DeferredNotifications.DeleteByMaint")
	defer span.End()

	stmt := table.MaintenanceDeferredNotifications.DELETE().
		WHERE(table.MaintenanceDeferredNotifications.MaintenanceID.EQ(postgres.UUID(maintID)))

	if _, err := stmt.ExecContext(ctx, s.db.Executor(ctx)); err != nil {
		return err
	}

	return nil
}

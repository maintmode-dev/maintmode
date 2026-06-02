package deferrednotifications

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// ListByMaint returns a maintenance's deferred-notification schedule, ordered by
// fire_at.
func (s *Store) ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.DeferredNotification, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.DeferredNotifications.ListByMaint")
	defer span.End()

	stmt := table.MaintenanceDeferredNotifications.
		SELECT(table.MaintenanceDeferredNotifications.AllColumns).
		WHERE(table.MaintenanceDeferredNotifications.MaintenanceID.EQ(postgres.UUID(maintID))).
		ORDER_BY(
			table.MaintenanceDeferredNotifications.FireAt.ASC(),
			table.MaintenanceDeferredNotifications.ID.ASC(),
		)

	rows := make([]*model.MaintenanceDeferredNotifications, 0)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows); err != nil {
		return nil, err
	}

	return lo.Map(rows, func(m *model.MaintenanceDeferredNotifications, _ int) *entity.DeferredNotification {
		return fromDB(m)
	}), nil
}

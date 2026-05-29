package notifytargets

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

func (s *Store) ListByMaint(ctx context.Context, maintID uuid.UUID) ([]*entity.NotifyTarget, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.MaintenanceNotifyTargets.ListByMaint")
	defer span.End()

	stmt := table.MaintenanceNotifyTargets.
		SELECT(table.MaintenanceNotifyTargets.AllColumns).
		WHERE(table.MaintenanceNotifyTargets.MaintenanceID.EQ(postgres.UUID(maintID))).
		ORDER_BY(
			table.MaintenanceNotifyTargets.CreatedAt.ASC(),
			table.MaintenanceNotifyTargets.Transport.ASC(),
			table.MaintenanceNotifyTargets.ChannelID.ASC(),
		)

	result := make([]*model.MaintenanceNotifyTargets, 0)
	if err := stmt.QueryContext(ctx, s.db.Executor(ctx), &result); err != nil {
		return nil, err
	}

	return lo.Map(result, func(item *model.MaintenanceNotifyTargets, _ int) *entity.NotifyTarget {
		return fromDB(item)
	}), nil
}

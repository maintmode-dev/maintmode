package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// GetMaintMentions returns the ids of the users mentioned by a maintenance.
// Ordering is by insertion (created_at, id) so the rendered mention line keeps
// a stable order across notifications for the same maintenance.
func (s *Store) GetMaintMentions(ctx context.Context, maintID uuid.UUID) ([]uuid.UUID, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.GetMaintMentions")
	defer span.End()

	stmt := table.MaintenanceMentions.
		SELECT(table.MaintenanceMentions.AllColumns).
		WHERE(table.MaintenanceMentions.MaintenanceID.EQ(postgres.UUID(maintID))).
		ORDER_BY(
			table.MaintenanceMentions.CreatedAt.ASC(),
			table.MaintenanceMentions.ID.ASC(),
		)

	mentions := make([]*model.MaintenanceMentions, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &mentions)
	if err != nil {
		return nil, err
	}

	return lo.Map(mentions, func(item *model.MaintenanceMentions, _ int) uuid.UUID {
		return item.UserID
	}), nil
}

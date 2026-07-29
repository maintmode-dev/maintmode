package maintenances

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) AddMentions(ctx context.Context, maintID uuid.UUID, userIDs []uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.AddMentions")
	defer span.End()

	if len(userIDs) == 0 {
		return nil
	}

	dbMentions := lo.Map(userIDs, func(id uuid.UUID, _ int) *model.MaintenanceMentions {
		return toDBMaintenanceMention(maintID, id)
	})

	stmt := table.MaintenanceMentions.
		INSERT(
			table.MaintenanceMentions.MaintenanceID,
			table.MaintenanceMentions.UserID,
		).
		MODELS(dbMentions).
		ON_CONFLICT(
			table.MaintenanceMentions.MaintenanceID,
			table.MaintenanceMentions.UserID,
		).DO_NOTHING()

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}

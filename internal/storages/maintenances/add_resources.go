package maintenances

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) AddResources(ctx context.Context, maintID uuid.UUID, resourceIDs []uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenances.AddResources")
	defer span.End()

	if len(resourceIDs) == 0 {
		return nil
	}

	dbResources := lo.Map(resourceIDs, func(id uuid.UUID, _ int) *model.MaintenanceResources {
		return toDBMaintenanceResource(maintID, id)
	})

	stmt := table.MaintenanceResources.
		INSERT(table.MaintenanceResources.AllColumns).
		MODELS(dbResources).
		ON_CONFLICT(
			table.MaintenanceResources.MaintenanceID,
			table.MaintenanceResources.ResourceID,
		).DO_NOTHING()

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}

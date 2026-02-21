package maintenances

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) AddResources(ctx context.Context, maintID uuid.UUID, resources []*entity.Resource) error {
	ctx = xlog.WithOperation(ctx, "store.Maintenances.AddResources")

	if len(resources) == 0 {
		return nil
	}

	dbResources := lo.Map(resources, func(item *entity.Resource, _ int) *model.MaintenanceResources {
		return toDBMaintenanceResource(maintID, item)
	})

	stmt := table.MaintenanceResources.
		INSERT(table.MaintenanceResources.AllColumns).
		MODELS(dbResources).
		ON_CONFLICT(
			table.MaintenanceResources.MaintenanceID,
			table.MaintenanceResources.ResourceID,
			table.MaintenanceResources.ResourceType,
		).DO_NOTHING()

	_, err := stmt.ExecContext(ctx, s.db.Executor(ctx))
	if err != nil {
		return err
	}

	return nil
}

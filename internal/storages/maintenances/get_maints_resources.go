package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/table"
)

func (s *Store) GetMaintResources(ctx context.Context, maintIDs []uuid.UUID) (map[uuid.UUID][]*entity.Resource, error) {
	ctx = xlog.WithOperation(ctx, "store.Maintenance.GetMaintResources")

	stmt := table.MaintenanceResources.
		SELECT(table.MaintenanceResources.AllColumns).
		WHERE(table.MaintenanceResources.MaintenanceID.IN(
			lo.Map(maintIDs, func(item uuid.UUID, _ int) postgres.Expression {
				return postgres.UUID(item)
			})...,
		))

	resources := make([]*model.MaintenanceResources, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &resources)
	if err != nil {
		return nil, err
	}

	maintResources := make(map[uuid.UUID][]*entity.Resource)
	for _, resource := range resources {
		maintResources[resource.MaintenanceID] = append(maintResources[resource.MaintenanceID],
			fromDBMaintenanceResource(resource),
		)
	}
	return maintResources, nil
}

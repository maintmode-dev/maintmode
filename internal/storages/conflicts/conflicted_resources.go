package conflicts

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/table"
)

func (s *Store) ConflictedResources(ctx context.Context, cmd *entity.ConflictResourcesQueryCmd) (map[uuid.UUID][]*entity.Resource, error) {
	ctx = xlog.WithOperation(ctx, "store.Conflicts.ConflictedResources")

	if len(cmd.MaintResourceIDs) == 0 || len(cmd.ConflictedMaintIDs) == 0 {
		return make(map[uuid.UUID][]*entity.Resource), nil
	}

	// SELECT
	//    mr.maintenance_id,
	//    mr.resource_id
	// FROM maintenance_resources mr
	// WHERE
	//    mr.maintenance_id = ANY(:conflict_ids)
	//  AND mr.resource_id = ANY(:draft_resource_ids);

	stmt := table.MaintenanceResources.
		SELECT(table.MaintenanceResources.AllColumns).
		WHERE(postgres.AND(
			table.MaintenanceResources.MaintenanceID.EQ(postgres.ANY(
				postgres.ARRAY(uuidsToPgUUID(cmd.ConflictedMaintIDs)...),
			)),
			table.MaintenanceResources.ResourceID.EQ(postgres.ANY(
				postgres.ARRAY(uuidsToPgUUID(cmd.MaintResourceIDs)...),
			)),
		))

	resources := make([]*model.MaintenanceResources, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &resources)
	if err != nil {
		return nil, err
	}

	conflictedResources := make(map[uuid.UUID][]*entity.Resource)
	for _, resource := range resources {
		conflictedResources[resource.MaintenanceID] = append(conflictedResources[resource.MaintenanceID],
			&entity.Resource{
				ID:   resource.ResourceID,
				Type: entity.ResourceType(resource.ResourceType),
			})
	}
	return conflictedResources, nil
}

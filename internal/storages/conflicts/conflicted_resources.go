package conflicts

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

func (s *Store) ConflictedResources(ctx context.Context, cmd *entity.ConflictResourcesQueryCmd) (map[uuid.UUID][]uuid.UUID, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Conflicts.ConflictedResources")
	defer span.End()

	if len(cmd.MaintResourceIDs) == 0 || len(cmd.ConflictedMaintIDs) == 0 {
		return make(map[uuid.UUID][]uuid.UUID), nil
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

	conflictedResources := make(map[uuid.UUID][]uuid.UUID)
	for _, resource := range resources {
		conflictedResources[resource.MaintenanceID] = append(conflictedResources[resource.MaintenanceID],
			resource.ResourceID,
		)
	}
	return conflictedResources, nil
}

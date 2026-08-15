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

	// Load-bearing, not a micro-optimization: an empty slice would reach
	// postgres.ARRAY below, which Jet serializes as an untyped `ARRAY[]` whose
	// element type Postgres cannot infer. GetConflicts calls this on every read,
	// so a maintenance with no conflicts is the common path.
	if len(cmd.ConflictedMaintIDs) == 0 {
		return make(map[uuid.UUID][]uuid.UUID), nil
	}

	// SELECT
	//    mr.maintenance_id,
	//    mr.resource_id
	// FROM maintenance_resources mr
	// WHERE
	//    mr.maintenance_id = ANY(:conflict_ids);
	//
	// Deliberately unfiltered by the querying maintenance's own resources: a
	// conflict reports what its maintenance touches, not what it shares with the
	// viewer. Intersecting here made the set depend on who was looking, which
	// left a global-scope maintenance — one that owns no resources — reading
	// every neighbor as resource-scoped with an empty resource list.
	stmt := table.MaintenanceResources.
		SELECT(table.MaintenanceResources.AllColumns).
		WHERE(
			table.MaintenanceResources.MaintenanceID.EQ(postgres.ANY(
				postgres.ARRAY(uuidsToPgUUID(cmd.ConflictedMaintIDs)...),
			)),
		)

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

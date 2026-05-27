package maintenances

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
)

// maintResourceRow is the row shape for the maintenance_resources ⨝ resources
// join — composes both jet models so jet can scan a joined SELECT directly.
type maintResourceRow struct {
	model.MaintenanceResources
	model.Resources
}

// GetMaintResources returns full resource details grouped by maintenance ID.
// It joins maintenance_resources with resources in a single query so callers
// don't need a follow-up lookup to resolve resource names/metadata.
func (s *Store) GetMaintResources(ctx context.Context, maintIDs []uuid.UUID) (map[uuid.UUID][]*entity.ResourceDetails, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.Maintenance.GetMaintResources")
	defer span.End()

	if len(maintIDs) == 0 {
		return map[uuid.UUID][]*entity.ResourceDetails{}, nil
	}

	stmt := table.MaintenanceResources.
		INNER_JOIN(table.Resources, table.Resources.ID.EQ(table.MaintenanceResources.ResourceID)).
		SELECT(
			table.MaintenanceResources.AllColumns,
			table.Resources.AllColumns,
		).
		WHERE(table.MaintenanceResources.MaintenanceID.EQ(
			postgres.ANY(postgres.ARRAY(uuidsToPgUUID(maintIDs)...))),
		)

	rows := make([]*maintResourceRow, 0)
	err := stmt.QueryContext(ctx, s.db.Executor(ctx), &rows)
	if err != nil {
		return nil, err
	}

	maintResources := make(map[uuid.UUID][]*entity.ResourceDetails)
	for _, row := range rows {
		maintResources[row.MaintenanceID] = append(maintResources[row.MaintenanceID],
			&entity.ResourceDetails{
				ID:          row.ID,
				Name:        row.Name,
				Description: row.Description,
				ExternalID:  row.ExternalID,
				CreatedAt:   row.CreatedAt,
				UpdatedAt:   row.UpdatedAt,
			},
		)
	}
	return maintResources, nil
}

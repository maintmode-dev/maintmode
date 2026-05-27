package conflictsnapshots

import (
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func toDBSnaphots(maintID uuid.UUID, snaphots []*entity.ConflictWithResources) []*model.MaintenanceConflictSnapshot {
	result := make([]*model.MaintenanceConflictSnapshot, 0, len(snaphots))

	for _, snapshot := range snaphots {
		if len(snapshot.Resources) > 0 {
			for _, resourceID := range snapshot.Resources {
				result = append(result, &model.MaintenanceConflictSnapshot{
					MaintenanceID:           maintID,
					ConflictedMaintenanceID: snapshot.MaintenanceID,
					ResourceID:              lo.ToPtr(resourceID),
					ConflictPeriod:          xtime.ToPgRange(entity.NewPeriod(snapshot.OverlapStart, snapshot.OverlapEnd)),
				})
			}
			continue
		}
		result = append(result, &model.MaintenanceConflictSnapshot{
			MaintenanceID:           maintID,
			ConflictedMaintenanceID: snapshot.MaintenanceID,
			ResourceID:              nil,
			ConflictPeriod:          xtime.ToPgRange(entity.NewPeriod(snapshot.OverlapStart, snapshot.OverlapEnd)),
		})
	}

	return result
}

func fromDBSnaphots(snaphots []*conflictSnapshots) []*entity.ConflictWithResources {
	// Group snapshots by conflicted_maintenance_id
	grouped := lo.GroupByMap(snaphots, func(item *conflictSnapshots) (uuid.UUID, *conflictSnapshots) {
		return item.ConflictedMaintenanceID, item
	})

	result := make([]*entity.ConflictWithResources, 0, len(grouped))
	for conflictedID, snapshots := range grouped {
		conflictedMaint := snapshots[0]

		// Collect resources
		resources := lo.FilterMap(snapshots, func(item *conflictSnapshots, _ int) (uuid.UUID, bool) {
			// collect only if OK
			ok := item.ResourceID != nil
			return lo.FromPtr(item.ResourceID), ok
		})

		conflictPeriod := xtime.FromPgRange(conflictedMaint.ConflictPeriod)
		result = append(result, &entity.ConflictWithResources{
			Conflict: &entity.Conflict{
				MaintenanceID: conflictedID,
				Title:         conflictedMaint.Title,
				OverlapStart:  conflictPeriod.Start,
				OverlapEnd:    lo.FromPtr(conflictPeriod.End),
				Scope:         entity.MaintenanceScope(conflictedMaint.Scope),
			},
			Resources: resources,
		})
	}

	return result
}

package calendar

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetConflicts(ctx context.Context, cmd *calendardto.ConflictQueryCmd) ([]*calendardto.Conflict, error) {
	ctx = xlog.WithOperation(ctx, "service.Calendar.GetConflicts")

	conflicts, err := s.conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
		MaintID:       cmd.MaintID,
		PlannedPeriod: cmd.PlannedPeriod,
		Scope:         cmd.Scope,
		ResourceIDs:   cmd.ResourceIDs,
	})
	if err != nil {
		return nil, err
	}

	conflictResourcesM := lo.SliceToMap(conflicts, func(item *entity.ConflictWithResources) (uuid.UUID, []*entity.Resource) {
		return item.MaintenanceID, item.Resources
	})

	resourcesDetails, err := s.getResourcesDetails(ctx, lo.Map(
		lo.Flatten(lo.Values(conflictResourcesM)), func(item *entity.Resource, _ int) uuid.UUID {
			return item.ID
		}),
	)
	if err != nil {
		return nil, err
	}

	return lo.Map(conflicts, func(item *entity.ConflictWithResources, _ int) *calendardto.Conflict {
		resources := lo.ValueOr(conflictResourcesM, item.MaintenanceID, []*entity.Resource{})
		return &calendardto.Conflict{
			MaintenanceID: item.MaintenanceID,
			Title:         item.Title,
			OverlapStart:  item.OverlapStart,
			OverlapEnd:    item.OverlapEnd,
			Scope:         item.Scope,
			Resources: lo.Map(resources, func(item *entity.Resource, _ int) *calendardto.MaintenanceResource {
				return &calendardto.MaintenanceResource{
					ID:   item.ID,
					Name: lo.ValueOr(resourcesDetails, item.ID, &entity.ResourceDetails{Name: "unknown resource"}).Name,
					Type: item.Type,
				}
			}),
		}
	}), nil
}

package calendar

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetConflicts(ctx context.Context, cmd *calendardto.ConflictQueryCmd) ([]*calendardto.Conflict, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Calendar.GetConflicts")
	defer span.End()

	conflicts, err := s.conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
		MaintID:       cmd.MaintID,
		PlannedPeriod: cmd.PlannedPeriod,
		Scope:         cmd.Scope,
		ResourceIDs:   cmd.ResourceIDs,
	})
	if err != nil {
		xlog.Error(ctx, "failed to get conflicts", xfield.Error(err))
		return nil, err
	}

	conflictResourcesM := lo.SliceToMap(conflicts, func(item *entity.ConflictWithResources) (uuid.UUID, []uuid.UUID) {
		return item.MaintenanceID, item.Resources
	})

	resourcesDetails, err := s.resolveResourceNames(ctx, lo.Flatten(lo.Values(conflictResourcesM)))
	if err != nil {
		xlog.Error(ctx, "failed to get resources details", xfield.Error(err))
		return nil, err
	}

	return lo.Map(conflicts, func(item *entity.ConflictWithResources, _ int) *calendardto.Conflict {
		resources := lo.ValueOr(conflictResourcesM, item.MaintenanceID, []uuid.UUID{})
		return &calendardto.Conflict{
			MaintenanceID: item.MaintenanceID,
			Title:         item.Title,
			OverlapStart:  item.OverlapStart,
			OverlapEnd:    item.OverlapEnd,
			Scope:         item.Scope,
			Resources: lo.Map(resources, func(id uuid.UUID, _ int) *calendardto.MaintenanceResource {
				// fallback to "unknown resource" if the resource is not found
				resDetails := lo.ValueOr(resourcesDetails, id, &entity.ResourceDetails{Name: "unknown resource"})
				return &calendardto.MaintenanceResource{
					ID:   id,
					Name: resDetails.Name,
				}
			}),
		}
	}), nil
}

// resolveResourceNames returns a map of resource ID → name for the given IDs.
// Used for projections (e.g., conflict-resources from a fingerprint snapshot)
// where only IDs are known and a name is needed for display.
func (s *Service) resolveResourceNames(ctx context.Context, resourceIDs []uuid.UUID) (map[uuid.UUID]*entity.ResourceDetails, error) {
	if len(resourceIDs) == 0 {
		return map[uuid.UUID]*entity.ResourceDetails{}, nil
	}

	details, err := s.resourcesStore.GetResources(ctx, resourceIDs)
	if err != nil {
		return nil, err
	}

	return lo.SliceToMap(details, func(item *entity.ResourceDetails) (uuid.UUID, *entity.ResourceDetails) {
		return item.ID, item
	}), nil
}

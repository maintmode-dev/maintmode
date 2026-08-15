package conflicts

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) GetConflicts(ctx context.Context, cmd *entity.ConflictQueryCmd) ([]*entity.ConflictWithResources, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Conflicts.GetConflicts")
	defer span.End()

	conflicts, err := s.conflictsStore.ConflictedMaints(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "failed to get conflicted maints", xfield.Error(err))
		return nil, err
	}

	conflictedResources, err := s.ConflictedResources(ctx, &entity.ConflictResourcesQueryCmd{
		ConflictedMaintIDs: lo.Map(conflicts, func(item *entity.Conflict, _ int) uuid.UUID {
			return item.MaintenanceID
		}),
	})
	if err != nil {
		xlog.Error(ctx, "failed to get conflicted resources", xfield.Error(err))
		return nil, err
	}

	return lo.Map(conflicts, func(item *entity.Conflict, _ int) *entity.ConflictWithResources {
		return &entity.ConflictWithResources{
			Conflict:  item,
			Resources: lo.ValueOr(conflictedResources, item.MaintenanceID, []uuid.UUID{}),
		}
	}), nil
}

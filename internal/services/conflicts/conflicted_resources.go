package conflicts

import (
	"context"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) ConflictedResources(ctx context.Context, cmd *entity.ConflictResourcesQueryCmd) (map[uuid.UUID][]*entity.Resource, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.Conflicts.ConflictedResources")
	defer span.End()

	resources, err := s.conflictsStore.ConflictedResources(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "failed to get conflicted resources", xfield.Error(err))
		return nil, err
	}

	return resources, nil
}

package conflicts

import (
	"context"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

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

	return s.attachResources(ctx, conflicts)
}

package conflicts

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) SaveSnapshot(ctx context.Context, cmd *entity.SaveConflictsSnapshotCmd) error {
	ctx = xlog.WithOperation(ctx, "service.Conflicts.SaveSnapshot")

	if err := s.conflictSnapshotsStore.Save(ctx, cmd.MaintID, cmd.ConflictSnapshot.Conflicts); err != nil {
		return fmt.Errorf("bulk insert conflict snapshots: %w", err)
	}

	return nil
}

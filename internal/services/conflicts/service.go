package conflicts

import (
	"context"

	"github.com/google/uuid"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
)

// ConflictSnapshotsStore persists and reads the conflict set frozen when a
// maintenance was approved. Declared consumer-side so tests can substitute a
// mock and drive the unreadable-snapshot degradation, which is otherwise
// unreachable; satisfied by *conflict_snapshots.Store.
type ConflictSnapshotsStore interface {
	GetSnapshots(ctx context.Context, maintID uuid.UUID) ([]*entity.ConflictWithResources, error)
	Save(ctx context.Context, maintID uuid.UUID, snapshots []*entity.ConflictWithResources) error
}

type Service struct {
	conflictsStore         *conflicts.Store
	conflictSnapshotsStore ConflictSnapshotsStore
}

func NewService(conflictsStore *conflicts.Store, conflictSnapshotsStore ConflictSnapshotsStore) *Service {
	return &Service{
		conflictsStore:         conflictsStore,
		conflictSnapshotsStore: conflictSnapshotsStore,
	}
}

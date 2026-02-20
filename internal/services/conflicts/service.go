package conflicts

import (
	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
)

type Service struct {
	conflictsStore         *conflicts.Store
	conflictSnapshotsStore *conflictsnapshots.Store
}

func NewService(conflictsStore *conflicts.Store, conflictSnapshotsStore *conflictsnapshots.Store) *Service {
	return &Service{
		conflictsStore:         conflictsStore,
		conflictSnapshotsStore: conflictSnapshotsStore,
	}
}

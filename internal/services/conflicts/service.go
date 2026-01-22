package conflicts

import "github.com/ruko1202/maintmode/internal/storages/conflicts"

type Service struct {
	conflictsStore *conflicts.Store
}

func NewService(conflictsStore *conflicts.Store) *Service {
	return &Service{
		conflictsStore: conflictsStore,
	}
}

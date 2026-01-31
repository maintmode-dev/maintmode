package calendar

import (
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
)

type Service struct {
	maintStore     *maintenances.Store
	resourcesStore *resources.Store
	conflictsSrv   *conflicts.Service
}

func NewService(
	maintStore *maintenances.Store,
	resourcesStore *resources.Store,
	conflictsSrv *conflicts.Service,
) *Service {
	return &Service{
		maintStore:     maintStore,
		resourcesStore: resourcesStore,
		conflictsSrv:   conflictsSrv,
	}
}

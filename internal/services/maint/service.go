package maint

import (
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

type Service struct {
	txManager      *dbtx.TxManager
	maintStore     *maintenances.Store
	resourcesStore *resources.Store

	conflictsSrv *conflicts.Service
}

func NewService(
	txManager *dbtx.TxManager,
	maintStore *maintenances.Store,
	resourcesStore *resources.Store,
	conflictsSrv *conflicts.Service,
) *Service {
	return &Service{
		txManager:      txManager,
		maintStore:     maintStore,
		resourcesStore: resourcesStore,
		conflictsSrv:   conflictsSrv,
	}
}

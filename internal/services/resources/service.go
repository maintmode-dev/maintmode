package resources

import (
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

type Service struct {
	txManager *dbtx.TxManager
	store     *resources.Store
}

func NewService(txManager *dbtx.TxManager, store *resources.Store) *Service {
	return &Service{
		txManager: txManager,
		store:     store,
	}
}

package notifytargets

import (
	"github.com/ruko1202/maintmode/internal/storages/notifychannel"
	"github.com/ruko1202/maintmode/internal/storages/notifytargets"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

type Service struct {
	txManager          *dbtx.TxManager
	channelCatalog     *notifychannel.Store
	notifyTargetsStore *notifytargets.Store
}

func NewService(
	txManager *dbtx.TxManager,
	channelCatalog *notifychannel.Store,
	notifyTargetsStore *notifytargets.Store,
) *Service {
	return &Service{
		txManager:          txManager,
		channelCatalog:     channelCatalog,
		notifyTargetsStore: notifyTargetsStore,
	}
}

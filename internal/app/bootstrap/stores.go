package bootstrap

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/goque"

	"github.com/ruko1202/maintmode/internal/storages/notifychannel"

	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/deferrednotifications"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/notifytargets"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Stores contains all storage layer dependencies
type Stores struct {
	TxManager             *dbtx.TxManager
	Maintenances          *maintenances.Store
	Resources             *resources.Store
	Conflicts             *conflicts.Store
	ConflictSnapshots     *conflictsnapshots.Store
	NotifyTargets         *notifytargets.Store
	DeferredNotifications *deferrednotifications.Store
	ChannelCatalog        *notifychannel.Store
	taskStorage           goque.TaskStorage
}

// NewStores creates and initializes all storage layer dependencies
func NewStores(
	db *sqlx.DB,
) (*Stores, error) {
	taskStorage, err := goque.NewStorage(db)
	if err != nil {
		return nil, fmt.Errorf("init goque storage: %w", err)
	}
	return &Stores{
		TxManager:             dbtx.NewTxManager(db),
		Maintenances:          maintenances.NewStore(db),
		Resources:             resources.NewStore(db),
		Conflicts:             conflicts.NewStore(db),
		ConflictSnapshots:     conflictsnapshots.NewStore(db),
		NotifyTargets:         notifytargets.NewStore(db),
		DeferredNotifications: deferrednotifications.NewStore(db),
		ChannelCatalog:        notifychannel.NewStore(db),
		taskStorage:           taskStorage,
	}, nil
}

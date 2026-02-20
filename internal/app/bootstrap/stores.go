package bootstrap

import (
	"github.com/jmoiron/sqlx"

	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Stores contains all storage layer dependencies
type Stores struct {
	TxManager         *dbtx.TxManager
	Maintenances      *maintenances.Store
	Resources         *resources.Store
	Conflicts         *conflicts.Store
	ConflictSnapshots *conflictsnapshots.Store
}

// NewStores creates and initializes all storage layer dependencies
func NewStores(db *sqlx.DB) *Stores {
	return &Stores{
		TxManager:         dbtx.NewTxManager(db),
		Maintenances:      maintenances.NewStore(db),
		Resources:         resources.NewStore(db),
		Conflicts:         conflicts.NewStore(db),
		ConflictSnapshots: conflictsnapshots.NewStore(db),
	}
}

package notifychannel

import (
	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Store is the messenger channel catalog backed by the messenger_channels
// table. It is the single source of truth across all instances: every Get/List
// reads live state, so a channel created on one pod is immediately visible and
// usable on every other pod with no restart and no cache to invalidate.
type Store struct {
	db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: dbtx.NewDB(db)}
}

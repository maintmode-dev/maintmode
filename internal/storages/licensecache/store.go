// Package licensecache is the storage owner of the license_cache singleton
// table: the last successful heartbeat response from the MaintMode Console. The
// license client upserts after every successful heartbeat; the enforcement
// points read the row at startup and on refresh. When Console is unreachable
// the row simply stays as-is — the instance runs on it indefinitely.
package licensecache

import (
	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Store reads and writes the single license_cache row.
type Store struct {
	db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: dbtx.NewDB(db)}
}

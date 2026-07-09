// Package datakey is the storage owner of the data_keys table: the wrapped
// data-encryption keys (DEKs) that protect integration secrets at rest. It holds
// the SQL for creating, reading, and re-wrapping DEK rows, plus the KEK-rotation
// orchestration that re-wraps every DEK under a new KEK in a single transaction.
package datakey

import (
	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Store is the data_keys catalog. Every read reflects live DB state; rotation
// runs its re-wrap inside a caller-provided transaction.
type Store struct {
	db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: dbtx.NewDB(db)}
}

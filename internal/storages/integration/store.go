// Package integration is the storage owner of the integration_settings table:
// the DB-backed registry of external-system connections (RUK-196). It holds the
// SQL for CRUD plus kind-keyed reads that the runtime resolver uses. config and
// secrets are stored as jsonb; the store marshals to/from the entity's
// map-shaped fields but never decrypts — secrets stay as stored ciphertext here.
package integration

import (
	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Store is the integration_settings catalog. Every read reflects live DB state so
// a change on one instance is visible to all; the resolver layers a cache on top.
type Store struct {
	db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: dbtx.NewDB(db)}
}

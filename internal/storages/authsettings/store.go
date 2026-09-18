// Package authsettings is the storage owner of the auth_settings table: one row
// per built-in sign-in method, carrying the flag that decides whether the login
// page offers it and whether the backend accepts it.
//
// Deliberately small. The table holds no config and no secrets -- that is the
// reason it is not a row in integration_settings -- so there is no marshaling
// layer here and no mapper beyond field copying.
package authsettings

import (
	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

// Store reads and writes the built-in method flags.
//
// Every read reflects live database state. There is no cache on top and that is
// a decision rather than an omission: a disabled method must stop working on
// every replica immediately, and a cached flag would leave a window in which an
// admin who just closed a sign-in path watches it keep answering.
type Store struct {
	db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: dbtx.NewDB(db)}
}

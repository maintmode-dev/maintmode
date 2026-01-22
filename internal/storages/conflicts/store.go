package conflicts

import (
	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

type Store struct {
	db *dbtx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: dbtx.NewDB(db)}
}

package dbtx

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type DB struct {
	db *sqlx.DB
}

func NewDB(db *sqlx.DB) *DB {
	return &DB{db: db}
}

func (t *DB) Executor(ctx context.Context) sqlx.ExtContext {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return t.db
}

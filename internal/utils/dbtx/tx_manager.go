package dbtx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

type TxManager struct {
	db       *sqlx.DB
	commit   func(ctx context.Context, tx *sqlx.Tx) error
	rollback func(ctx context.Context, tx *sqlx.Tx) error
}

func NewTxManager(db *sqlx.DB) *TxManager {
	return &TxManager{
		db:       db,
		commit:   commit,
		rollback: rollback,
	}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	ctx, span := xlog.WithOperationSpan(ctx, "txManager.WithinTx")
	defer span.End()

	// Reentrant: if a transaction is already active in the context, join it
	// instead of opening a second, independent one. Begin/commit/rollback stay
	// with the outermost WithinTx — a nested call only runs fn against the
	// existing tx and propagates its error so the outer call rolls back the
	// whole unit. This lets a service that wraps work in a transaction call
	// other transactional services and have them all commit atomically.
	//
	// There are no savepoints: a nested call cannot roll back just its own part
	// while the outer transaction continues. Every caller propagates the error
	// upward (none swallows it to "continue after a failed sub-step"), so this
	// matches existing usage; do not rely on partial rollback.
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, beginTxErr := m.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
	if beginTxErr != nil {
		return beginTxErr
	}

	ctx = context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if recErr := recover(); recErr != nil {
			xlog.Error(ctx, "panic recovery when execute the transaction", xfield.Any("panic", recErr))
			err = errors.Join(err, fmt.Errorf("panic recovery: %v", recErr))
		}

		if err != nil {
			if rollbackErr := m.rollback(ctx, tx); rollbackErr != nil {
				err = errors.Join(err, rollbackErr)
				xlog.Error(ctx, "raise error when rollback the transaction", xfield.Error(err))
			}
		}
	}()

	if fnErr := fn(ctx); fnErr != nil {
		err = errors.Join(err, fnErr)
		xlog.Error(ctx, "raise error when execute the transaction", xfield.Error(err))
		return err
	}

	if commitErr := m.commit(ctx, tx); commitErr != nil {
		err = errors.Join(err, commitErr)
		xlog.Error(ctx, "raise error when commit the transaction", xfield.Error(err))
		return err
	}

	return err
}

func rollback(ctx context.Context, tx *sqlx.Tx) error {
	if err := tx.Rollback(); err != nil {
		xlog.Error(ctx, "failed to rollback the transaction", xfield.Error(err))
		return err
	}
	return nil
}

func commit(ctx context.Context, tx *sqlx.Tx) error {
	if err := tx.Commit(); err != nil {
		xlog.Error(ctx, "failed to commit the transaction", xfield.Error(err))
		return err
	}
	return nil
}

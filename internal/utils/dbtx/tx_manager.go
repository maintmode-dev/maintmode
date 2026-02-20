package dbtx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"
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
	tx, beginTxErr := m.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
	if beginTxErr != nil {
		return beginTxErr
	}

	ctx = context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if recErr := recover(); recErr != nil {
			xlog.Error(ctx, "panic recovery when execute the transaction", zap.Any("panic", recErr))
			err = errors.Join(err, fmt.Errorf("panic recovery: %v", recErr))
		}

		if err != nil {
			if rollbackErr := m.rollback(ctx, tx); rollbackErr != nil {
				err = errors.Join(err, rollbackErr)
				xlog.Error(ctx, "raise error when rollback the transaction", zap.Error(err))
			}
		}
	}()

	if fnErr := fn(ctx); fnErr != nil {
		err = errors.Join(err, fnErr)
		xlog.Error(ctx, "raise error when execute the transaction", zap.Error(err))
		return err
	}

	if commitErr := m.commit(ctx, tx); commitErr != nil {
		err = errors.Join(err, commitErr)
		xlog.Error(ctx, "raise error when commit the transaction", zap.Error(err))
		return err
	}

	return err
}

func rollback(ctx context.Context, tx *sqlx.Tx) error {
	if err := tx.Rollback(); err != nil {
		xlog.Error(ctx, "failed to rollback the transaction", zap.Error(err))
		return err
	}
	return nil
}

func commit(ctx context.Context, tx *sqlx.Tx) error {
	if err := tx.Commit(); err != nil {
		xlog.Error(ctx, "failed to commit the transaction", zap.Error(err))
		return err
	}
	return nil
}

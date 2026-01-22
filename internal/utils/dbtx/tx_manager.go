package dbtx

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"
)

type TxManager struct {
	db *sqlx.DB
}

func NewTxManager(db *sqlx.DB) *TxManager {
	return &TxManager{db: db}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
	if err != nil {
		return err
	}

	ctx = context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if recErr := recover(); recErr != nil {
			xlog.Error(ctx, "panic recovery when execute the transaction", zap.Any("panic", recErr))
		}
	}()

	if err := fn(ctx); err != nil {
		xlog.Error(ctx, "raise error when execute the transaction", zap.Error(err))
		_ = rollback(ctx, tx)
		return err
	}

	return commit(ctx, tx)
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

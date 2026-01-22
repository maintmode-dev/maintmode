// Package pg provides PostgreSQL database connection management.
// It includes connection pooling configuration and health checking.
package pg

import (
	"context"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" //nolint:revive
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/config"
)

// NewDBConn creates a new database connection with configured settings.
func NewDBConn(ctx context.Context, cfg *config.DB) (*sqlx.DB, error) {
	xlog.Info(ctx, "Using config:",
		zap.String("driver", cfg.Driver),
		zap.Int("max_open_conn", cfg.MaxOpenConn),
		zap.Int("max_idle_conn", cfg.MaxIdleConn),
		zap.Duration("conn_max_lifetime", cfg.ConnMaxLifetime),
		zap.Duration("conn_max_idle_time", cfg.ConnMaxIdleTime),
	)

	db, err := sqlx.ConnectContext(ctx, cfg.Driver, cfg.DSN)
	if err != nil {
		xlog.Error(ctx, "failed to open db connection", zap.Error(err))
		return nil, err
	}
	db.SetMaxOpenConns(cfg.MaxOpenConn)
	db.SetMaxIdleConns(cfg.MaxIdleConn)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	xlog.Info(ctx, "db connection opened")
	return db, nil
}

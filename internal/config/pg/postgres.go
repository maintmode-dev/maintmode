// Package pg provides PostgreSQL database connection management.
// It includes connection pooling configuration and health checking.
package pg

import (
	"context"
	"fmt"
	"time"

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

	db, err := sqlx.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		xlog.Error(ctx, "failed to open db connection", zap.Error(err))
		return nil, err
	}
	db.SetMaxOpenConns(cfg.MaxOpenConn)
	db.SetMaxIdleConns(cfg.MaxIdleConn)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := waitOpenConn(ctx, db, 10*time.Second); err != nil {
		xlog.Error(ctx,
			fmt.Sprintf("waiting for connection opening failed [timeout: %s]", 10*time.Second),
			zap.Error(err),
		)
		return nil, err
	}

	xlog.Info(ctx, "db connection opened")
	return db, nil
}

func waitOpenConn(ctx context.Context, db *sqlx.DB, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			xlog.Info(ctx, "db ping")
			err := db.PingContext(ctx)
			if err != nil {
				xlog.Error(ctx, "failed to ping db connection", zap.Error(err))
				continue
			}
			xlog.Info(ctx, "db pong")
			return nil
		}
	}
}

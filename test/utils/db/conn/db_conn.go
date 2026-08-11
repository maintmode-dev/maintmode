package testdbconnutils

import (
	"context"

	"github.com/jmoiron/sqlx"
	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/config/valkey"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/pg"
)

func NewDB(cfg *config.AppConfig) *sqlx.DB {
	ctx := context.Background()

	conn, err := pg.NewDBConn(ctx, &cfg.DB)
	if err != nil {
		xlog.Panic(ctx, "open db conn failed", xfield.Error(err))
	}

	return conn
}

func NewValkeyClient(cfg *config.AppConfig) *valkeyDB.Client {
	ctx := context.Background()

	client, err := valkey.NewValkey(ctx, &cfg.Valkey)
	if err != nil {
		xlog.Panic(ctx, "init valkey client failed", xfield.Error(err))
	}

	return client
}

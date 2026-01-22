package testdbconnutils

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/pg"
)

func NewDB() *sqlx.DB {
	ctx := context.Background()
	viper.AutomaticEnv()

	conn, err := pg.NewDBConn(ctx, &config.DB{
		DSN:             viper.GetString("DB_DSN"),
		Driver:          viper.GetString("DB_DRIVER"),
		MaxIdleConn:     viper.GetInt("DB_MAX_IDLE_CONNS"),
		MaxOpenConn:     viper.GetInt("DB_MAX_OPEN_CONNS"),
		ConnMaxIdleTime: viper.GetDuration("DB_CONNECTION_MAX_IDLE_TIME"),
		ConnMaxLifetime: viper.GetDuration("DB_CONNECTIONS_MAX_LIFETIME"),
	})
	if err != nil {
		xlog.Panic(ctx, "open db conn failed", zap.Error(err))
	}

	return conn
}

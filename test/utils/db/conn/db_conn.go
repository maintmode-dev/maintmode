package testdbconnutils

import (
	"context"

	"github.com/jmoiron/sqlx"
	redisDB "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/spf13/viper"

	"github.com/ruko1202/maintmode/internal/config/redis"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/pg"
)

func NewDB() *sqlx.DB {
	ctx := context.Background()
	viper.AutomaticEnv()

	cfg := config.LoadAppConfig()

	conn, err := pg.NewDBConn(ctx, &cfg.DB)
	if err != nil {
		xlog.Panic(ctx, "open db conn failed", xfield.Error(err))
	}

	return conn
}

func NewRedisClient() *redisDB.Client {
	ctx := context.Background()
	viper.AutomaticEnv()

	client, err := redis.NewRedis(ctx, &config.Redis{
		Address:  viper.GetString("REDIS_ADDR"),
		Password: viper.GetString("REDIS_PASSWORD"),
		DB:       viper.GetInt("REDIS_DB"),
	})
	if err != nil {
		xlog.Panic(ctx, "init redis client failed", xfield.Error(err))
	}

	return client
}

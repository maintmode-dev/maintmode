package redis

import (
	"context"
	"fmt"
	"time"

	redisDB "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/config"
)

func NewRedis(ctx context.Context, cfg *config.Redis) (*redisDB.Client, error) {
	client := redisDB.NewClient(&redisDB.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := ping(ctx, client, time.Second); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	xlog.Info(ctx, "connected to Redis")
	return client, nil
}

func ping(ctx context.Context, client *redisDB.Client, freq time.Duration) error {
	var lastErr error
	for i := 0; i < 10; i++ {
		xlog.Info(ctx, "pinging redis")
		err := client.Ping(ctx).Err()
		if err == nil {
			xlog.Info(ctx, "pinged redis: OK")
			return nil
		}
		lastErr = err
		xlog.Error(ctx, "pinged redis: ERROR", xfield.Error(err))
		time.Sleep(freq)
	}

	return fmt.Errorf("failed to ping redis after 10 retries: %w", lastErr)
}

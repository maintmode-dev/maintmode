package valkey

import (
	"context"
	"fmt"
	"time"

	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/config"
)

func NewValkey(ctx context.Context, cfg *config.Valkey) (*valkeyDB.Client, error) {
	client := valkeyDB.NewClient(&valkeyDB.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := ping(ctx, client, time.Second); err != nil {
		return nil, fmt.Errorf("ping valkey: %w", err)
	}

	xlog.Info(ctx, "connected to Valkey")
	return client, nil
}

func ping(ctx context.Context, client *valkeyDB.Client, freq time.Duration) error {
	var lastErr error
	for i := 0; i < 10; i++ {
		xlog.Info(ctx, "pinging valkey")
		err := client.Ping(ctx).Err()
		if err == nil {
			xlog.Info(ctx, "pinged valkey: OK")
			return nil
		}
		lastErr = err
		xlog.Error(ctx, "pinged valkey: ERROR", xfield.Error(err))
		time.Sleep(freq)
	}

	return fmt.Errorf("failed to ping valkey after 10 retries: %w", lastErr)
}

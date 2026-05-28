package server

import (
	"github.com/labstack/echo/v5/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/server/ratelimiter"
	"github.com/ruko1202/maintmode/internal/server/ratelimiter/redisratelimiter"
)

func NewRateLimiter(appName string, rdb *redis.Client, cfg config.RateLimiterConfig) *ratelimiter.HybridRateLimiter {
	// echo's in-memory fallback is a token bucket whose Rate is in
	// requests/second, so convert the per-minute config.
	fallback := middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
		Rate:      float64(cfg.RequestsPerMinute) / 60,
		Burst:     cfg.Burst,
		ExpiresIn: cfg.ExpiresIn,
	})

	return ratelimiter.NewHybridRateLimiter(appName,
		redisratelimiter.NewRedisLimiter(rdb,
			redisratelimiter.WithWindowMinute(cfg.RequestsPerMinute),
		),
		ratelimiter.WithTimeout(cfg.Timeout),
		ratelimiter.WithFallbackLimiter(ratelimiter.LimiterNoCtx(fallback.Allow)),
	)
}

package redisratelimiter

import (
	"context"
	"fmt"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

type RedisLimiterOpts func(*RedisLimiter)

const defaultRate = 30 // 30 requests per window

type RedisLimiter struct {
	limiter *redis_rate.Limiter
	limit   redis_rate.Limit
}

func NewRedisLimiter(
	rdb *redis.Client,
	opts ...RedisLimiterOpts,
) *RedisLimiter {
	l := &RedisLimiter{
		limiter: redis_rate.NewLimiter(rdb),
		limit:   redis_rate.PerMinute(defaultRate),
	}
	for _, opt := range opts {
		opt(l)
	}

	return l
}

func (r *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	res, err := r.limiter.Allow(ctx, key, r.limit)
	if err != nil {
		return false, fmt.Errorf("failed to bump rate limiter: %w", err)
	}

	return res.Allowed > 0, nil
}

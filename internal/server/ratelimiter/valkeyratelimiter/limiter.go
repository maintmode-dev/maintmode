package valkeyratelimiter

import (
	"context"
	"fmt"

	"github.com/go-redis/redis_rate/v10"
	valkeylib "github.com/redis/go-redis/v9"
)

type ValkeyLimiterOpts func(*ValkeyLimiter)

const defaultRate = 30 // 30 requests per window

type ValkeyLimiter struct {
	limiter *redis_rate.Limiter
	limit   redis_rate.Limit
}

func NewValkeyLimiter(
	rdb *valkeylib.Client,
	opts ...ValkeyLimiterOpts,
) *ValkeyLimiter {
	l := &ValkeyLimiter{
		limiter: redis_rate.NewLimiter(rdb),
		limit:   redis_rate.PerMinute(defaultRate),
	}
	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Limit reports the window the constructor and its options settled on. Callers
// that pair this limiter with a second one — the in-memory bucket standing in
// during a Valkey outage — need it to size that bucket identically, so an outage
// changes where the decision is made and not what the limit is.
func (r *ValkeyLimiter) Limit() redis_rate.Limit {
	return r.limit
}

func (r *ValkeyLimiter) Allow(ctx context.Context, key string) (bool, error) {
	res, err := r.limiter.Allow(ctx, key, r.limit)
	if err != nil {
		return false, fmt.Errorf("failed to bump rate limiter: %w", err)
	}

	return res.Allowed > 0, nil
}

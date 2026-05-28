package ratelimiter

import (
	"context"
	"fmt"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/metrics"
)

const (
	keyPrefix      = "ratelimit:"
	defaultTimeout = 200 * time.Millisecond
)

type Limiter interface {
	Allow(ctx context.Context, identifier string) (bool, error)
}

type HybridRateLimiter struct {
	appNamePrefix   string
	baseLimiter     Limiter
	fallbackLimiter Limiter
	timeout         time.Duration
}

func NewHybridRateLimiter(
	appName string,
	limiter Limiter,
	opts ...Option,
) *HybridRateLimiter {
	s := &HybridRateLimiter{
		appNamePrefix:   fmt.Sprintf("%s:", appName),
		timeout:         defaultTimeout,
		baseLimiter:     limiter,
		fallbackLimiter: NewNoopLimiter(),
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *HybridRateLimiter) Allow(identifier string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	allow, err := s.baseLimiter.Allow(ctx, keyPrefix+s.appNamePrefix+identifier)
	if err != nil {
		metrics.RateLimiterFallback(ctx)
		xlog.Warn(ctx, "rate limiter: redis unavailable, using fallback", xfield.Error(err))
		return s.fallbackLimiter.Allow(ctx, identifier)
	}

	return allow, nil
}

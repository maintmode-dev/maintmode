package ratelimiter

import "context"

type LimiterNoCtx func(identifier string) (bool, error)

func (l LimiterNoCtx) Allow(_ context.Context, identifier string) (bool, error) {
	return l(identifier)
}

type NoopLimiter struct{}

func NewNoopLimiter() *NoopLimiter { return &NoopLimiter{} }

// Allow permits an operation without applying any rate limiting
// allowed = 1 means always allowed
func (NoopLimiter) Allow(context.Context, string) (bool, error) { return true, nil }

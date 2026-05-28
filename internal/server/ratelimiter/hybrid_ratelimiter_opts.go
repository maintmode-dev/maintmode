package ratelimiter

import "time"

// Option overrides a default. Non-positive / nil values are ignored.
type Option func(*HybridRateLimiter)

// WithTimeout sets the per-call Redis deadline.
func WithTimeout(d time.Duration) Option {
	return func(s *HybridRateLimiter) {
		if d > 0 {
			s.timeout = d
		}
	}
}

// WithFallbackLimiter injects the per-replica fallback.
func WithFallbackLimiter(fl Limiter) Option {
	return func(s *HybridRateLimiter) {
		if fl != nil {
			s.fallbackLimiter = fl
		}
	}
}

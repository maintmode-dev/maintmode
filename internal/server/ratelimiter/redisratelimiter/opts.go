package redisratelimiter

import "github.com/go-redis/redis_rate/v10"

// Both options ignore a non-positive rate and leave the constructor's default in
// place. Zero is not "no limit" to redis_rate — PerMinute(0) is Limit{Rate:0,
// Burst:0}, a bucket that denies every request — and callers reach here straight
// from config, which carries no viper defaults, so a missing or half-filled yaml
// block arrives as a bare Go zero. Applying it would turn a forgotten config
// line into a route answering 100% HTTP 429 off a clean startup: fail-closed
// inside machinery that otherwise fails open (a Redis outage degrades to the
// in-memory fallback). An operator's own value always wins; only a meaningless
// one is refused.

func WithWindowSecond(rate int) RedisLimiterOpts {
	return func(r *RedisLimiter) {
		if rate <= 0 {
			return
		}
		r.limit = redis_rate.PerSecond(rate)
	}
}

func WithWindowMinute(rate int) RedisLimiterOpts {
	return func(r *RedisLimiter) {
		if rate <= 0 {
			return
		}
		r.limit = redis_rate.PerMinute(rate)
	}
}

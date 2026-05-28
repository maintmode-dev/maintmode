package redisratelimiter

import "github.com/go-redis/redis_rate/v10"

func WithWindowSecond(rate int) RedisLimiterOpts {
	return func(r *RedisLimiter) {
		r.limit = redis_rate.PerSecond(rate)
	}
}

func WithWindowMinute(rate int) RedisLimiterOpts {
	return func(r *RedisLimiter) {
		r.limit = redis_rate.PerMinute(rate)
	}
}

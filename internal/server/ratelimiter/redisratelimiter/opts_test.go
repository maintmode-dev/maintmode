package redisratelimiter

import (
	"testing"

	"github.com/go-redis/redis_rate/v10"
	"github.com/stretchr/testify/require"
)

// TestWindowOptsIgnoreNonPositiveRate pins that a zero or negative rate leaves
// the package default in place instead of installing it.
//
// Without this guard the options are a fail-CLOSED trap in an otherwise
// fail-open design. Config blocks carry no viper defaults, so an absent or
// half-filled yaml block unmarshals to bare Go zeros, and the caller passes that
// zero straight through. redis_rate.PerMinute(0) is Limit{Rate:0, Burst:0} — a
// bucket that denies EVERY request. The route would answer 100% HTTP 429 in
// production off a clean startup with no alert, while the rest of the limiter
// machinery (Redis outage → in-memory fallback) fails open.
//
// Degrading to the package default is the safer half of the trade: a forgotten
// block yields a conservative cap rather than an outage, and the operator's own
// value always wins when it is meaningful.
func TestWindowOptsIgnoreNonPositiveRate(t *testing.T) {
	t.Parallel()

	packageDefault := redis_rate.PerMinute(defaultRate)

	t.Run("a zero rate keeps the default", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, packageDefault, NewRedisLimiter(nil, WithWindowMinute(0)).limit)
		require.Equal(t, packageDefault, NewRedisLimiter(nil, WithWindowSecond(0)).limit)
	})

	t.Run("a negative rate keeps the default", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, packageDefault, NewRedisLimiter(nil, WithWindowMinute(-1)).limit)
		require.Equal(t, packageDefault, NewRedisLimiter(nil, WithWindowSecond(-1)).limit)
	})

	// The control: without it the assertions above would also hold if the options
	// had stopped working altogether.
	t.Run("a meaningful rate is applied", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, redis_rate.PerMinute(120), NewRedisLimiter(nil, WithWindowMinute(120)).limit)
		require.Equal(t, redis_rate.PerSecond(5), NewRedisLimiter(nil, WithWindowSecond(5)).limit)
	})
}

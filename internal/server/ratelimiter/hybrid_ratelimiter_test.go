package ratelimiter

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeLimiter is a deterministic Limiter for tests: it returns a fixed
// allow/err and counts how many times it was asked.
type fakeLimiter struct {
	allow  bool
	err    error
	called atomic.Int64
}

func (f *fakeLimiter) Allow(_ context.Context, _ string) (bool, error) {
	f.called.Add(1)
	return f.allow, f.err
}

func TestHybridRateLimiter_Defaults(t *testing.T) {
	s := NewHybridRateLimiter("auth", &fakeLimiter{allow: true})
	require.Equal(t, defaultTimeout, s.timeout)
	require.IsType(t, &NoopLimiter{}, s.fallbackLimiter, "default fallback must be the fail-open noop")
}

func TestHybridRateLimiter_Allow_BaseAllows(t *testing.T) {
	base := &fakeLimiter{allow: true}
	fallback := &fakeLimiter{allow: false}
	s := NewHybridRateLimiter("auth", base, WithFallbackLimiter(fallback))

	ok, err := s.Allow("ip-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, 1, base.called.Load())
	require.EqualValues(t, 0, fallback.called.Load(), "fallback must not be touched on the healthy path")
}

func TestHybridRateLimiter_Allow_BaseDenies(t *testing.T) {
	base := &fakeLimiter{allow: false}
	fallback := &fakeLimiter{allow: true} // would allow if asked — proves it isn't
	s := NewHybridRateLimiter("auth", base, WithFallbackLimiter(fallback))

	ok, err := s.Allow("ip-1")
	require.NoError(t, err)
	require.False(t, ok, "a base deny must not fall through to the fallback")
	require.EqualValues(t, 0, fallback.called.Load())
}

func TestHybridRateLimiter_Allow_FallsBackOnError(t *testing.T) {
	base := &fakeLimiter{err: errors.New("dial tcp: connection refused")}
	fallback := &fakeLimiter{allow: true}
	s := NewHybridRateLimiter("auth", base, WithFallbackLimiter(fallback))

	ok, err := s.Allow("ip-1")
	require.NoError(t, err, "a base error must be swallowed and deferred to the fallback")
	require.True(t, ok)
	require.EqualValues(t, 1, fallback.called.Load(), "fallback is consulted only when base errors")
}

func TestHybridRateLimiter_Allow_FallbackDenies(t *testing.T) {
	base := &fakeLimiter{err: errors.New("timeout")}
	fallback := &fakeLimiter{allow: false}
	s := NewHybridRateLimiter("auth", base, WithFallbackLimiter(fallback))

	ok, err := s.Allow("ip-1")
	require.NoError(t, err)
	require.False(t, ok, "fallback's deny must be respected")
}

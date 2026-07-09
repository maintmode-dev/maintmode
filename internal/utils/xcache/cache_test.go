package xcache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCache_SetGetInvalidate(t *testing.T) {
	t.Parallel()
	c := New[string, string](time.Minute)

	_, ok := c.Get("slack")
	require.False(t, ok, "empty cache misses")

	c.Set("slack", "v1", c.Generation("slack"))
	got, ok := c.Get("slack")
	require.True(t, ok, "hit after set")
	require.Equal(t, "v1", got)

	c.Invalidate("slack")
	_, ok = c.Get("slack")
	require.False(t, ok, "miss after invalidate")
}

// A Set carrying a generation snapshot from before an Invalidate must be
// dropped, so a reader that loaded pre-write source state cannot repopulate
// the cache after the write already invalidated it (the read-then-cache vs
// invalidate race).
func TestCache_StaleSetDroppedAfterInvalidate(t *testing.T) {
	t.Parallel()
	c := New[string, string](time.Minute)

	// The reader snapshots the generation, then a concurrent write invalidates.
	snap := c.Generation("slack")
	c.Invalidate("slack")

	// The reader's Set arrives late with the stale snapshot: it must not cache.
	c.Set("slack", "stale", snap)
	_, ok := c.Get("slack")
	require.False(t, ok, "a set with a pre-invalidate generation must be dropped")

	// A fresh read (post-invalidate snapshot) caches normally.
	c.Set("slack", "fresh", c.Generation("slack"))
	got, ok := c.Get("slack")
	require.True(t, ok, "a set with the current generation caches")
	require.Equal(t, "fresh", got)
}

// Generations are per-key: invalidating one key must not drop a concurrent
// set for another.
func TestCache_GenerationsArePerKey(t *testing.T) {
	t.Parallel()
	c := New[string, int](time.Minute)

	snap := c.Generation("telegram")
	c.Invalidate("slack") // unrelated key

	c.Set("telegram", 7, snap)
	got, ok := c.Get("telegram")
	require.True(t, ok, "an invalidate on another key must not drop this set")
	require.Equal(t, 7, got)
}

func TestCache_TTLExpiry(t *testing.T) {
	t.Parallel()
	c := New[string, string](30 * time.Second)
	base := time.Unix(1_000_000, 0)
	c.now = func() time.Time { return base }

	c.Set("slack", "v", c.Generation("slack"))

	// Within TTL: hit.
	c.now = func() time.Time { return base.Add(29 * time.Second) }
	_, ok := c.Get("slack")
	require.True(t, ok)

	// Past TTL: miss (and the entry is evicted).
	c.now = func() time.Time { return base.Add(31 * time.Second) }
	_, ok = c.Get("slack")
	require.False(t, ok, "expired entry must miss")
}

// The zero value of V must be distinguishable from a miss only via ok.
func TestCache_ZeroValueRoundTrips(t *testing.T) {
	t.Parallel()
	c := New[string, int](time.Minute)
	c.Set("k", 0, c.Generation("k"))
	got, ok := c.Get("k")
	require.True(t, ok)
	require.Zero(t, got)
}

// Concurrent Get/Set/Invalidate under the race detector.
func TestCache_ConcurrentAccessIsSafe(t *testing.T) {
	t.Parallel()
	c := New[int, int](time.Minute)

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			key := i % 4
			snap := c.Generation(key)
			c.Set(key, i, snap)
			c.Get(key)
			c.Invalidate(key)
		}(i)
	}
	wg.Wait()

	// Every key was invalidated at least once by the loop above; the generation
	// must reflect it (and reaching here means no data race fired).
	require.GreaterOrEqual(t, c.Generation(0), uint64(1))
}

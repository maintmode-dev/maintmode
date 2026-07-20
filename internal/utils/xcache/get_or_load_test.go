package xcache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetOrLoad_CachesAndServesFromCache(t *testing.T) {
	t.Parallel()

	c := New[string, int](time.Hour)
	var calls atomic.Int64

	load := func() (int, error) {
		calls.Add(1)
		return 42, nil
	}

	v, err := c.GetOrLoad("k", load)
	require.NoError(t, err)
	require.Equal(t, 42, v)

	v, err = c.GetOrLoad("k", load)
	require.NoError(t, err)
	require.Equal(t, 42, v)

	require.EqualValues(t, 1, calls.Load(), "second call must be served from cache")
}

func TestGetOrLoad_CoalescesConcurrentMisses(t *testing.T) {
	t.Parallel()

	c := New[string, int](time.Hour)
	var calls atomic.Int64

	const n = 20
	release := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(n)

	//nolint:unparam // the loader signature is dictated by GetOrLoad; this one never errors.
	load := func() (int, error) {
		calls.Add(1)
		<-release // hold the flight open until every goroutine is waiting
		return 7, nil
	}

	var got sync.WaitGroup
	got.Add(n)
	results := make([]int, n)
	for i := range n {
		go func() {
			defer got.Done()
			ready.Done()
			v, err := c.GetOrLoad("k", load)
			require.NoError(t, err)
			results[i] = v
		}()
	}

	ready.Wait()   // all n goroutines are inside GetOrLoad
	close(release) // let the single in-flight loader finish
	got.Wait()

	require.EqualValues(t, 1, calls.Load(), "concurrent misses must coalesce into one load")
	for _, v := range results {
		require.Equal(t, 7, v)
	}
}

func TestGetOrLoad_ErrorNotCached(t *testing.T) {
	t.Parallel()

	c := New[string, int](time.Hour)
	boom := errors.New("boom")

	_, err := c.GetOrLoad("k", func() (int, error) { return 0, boom })
	require.ErrorIs(t, err, boom)

	// A failed load leaves the cache empty, so the next call retries.
	v, err := c.GetOrLoad("k", func() (int, error) { return 1, nil })
	require.NoError(t, err)
	require.Equal(t, 1, v)

	_, ok := c.Get("k")
	require.True(t, ok, "the successful retry must be cached")
}

func TestGetOrLoad_InvalidateMidFlightNotCached(t *testing.T) {
	t.Parallel()

	c := New[string, int](time.Hour)

	// The loader invalidates the key while it is "loading", simulating a write
	// that commits mid-flight. The built value is returned but must not be
	// cached (its generation is stale).
	v, err := c.GetOrLoad("k", func() (int, error) {
		c.Invalidate("k")
		return 99, nil
	})
	require.NoError(t, err)
	require.Equal(t, 99, v)

	_, ok := c.Get("k")
	require.False(t, ok, "a value invalidated mid-flight must not be cached")
}

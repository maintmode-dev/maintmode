package distributedlock_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/storages/distributedlock"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

const lockTTL = time.Minute

func newKey() string {
	return "test:" + xuuid.NewString()
}

func TestAcquire(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := distributedlock.NewStore(valkey)

	t.Run("free key is acquired, a held one is busy", func(t *testing.T) {
		t.Parallel()
		key := newKey()

		lock, err := store.Acquire(ctx, key, lockTTL)
		require.NoError(t, err)
		require.NotNil(t, lock)

		_, err = store.Acquire(ctx, key, lockTTL)
		require.ErrorIs(t, err, apperr.ErrLockBusy)
	})

	// A holder that dies never releases: without the TTL the key would refuse
	// every later refresh of that token for good.
	t.Run("the lock carries the requested expiry", func(t *testing.T) {
		t.Parallel()
		key := newKey()

		lock, err := store.Acquire(ctx, key, lockTTL)
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Release(context.Background(), lock) })

		ttl, err := valkey.PTTL(ctx, "lock:"+key).Result()
		require.NoError(t, err)
		require.Greater(t, ttl, lockTTL/2, "the lock must carry the requested expiry")
		require.LessOrEqual(t, ttl, lockTTL)
	})
}

func TestRelease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := distributedlock.NewStore(valkey)

	t.Run("owner release frees the key", func(t *testing.T) {
		t.Parallel()
		key := newKey()

		lock, err := store.Acquire(ctx, key, lockTTL)
		require.NoError(t, err)
		require.NoError(t, store.Release(ctx, lock))

		next, err := store.Acquire(ctx, key, lockTTL)
		require.NoError(t, err)
		require.NoError(t, store.Release(ctx, next))
	})

	t.Run("release after expiry leaves the next owner's lock alone", func(t *testing.T) {
		t.Parallel()
		key := newKey()

		stale, err := store.Acquire(ctx, key, lockTTL)
		require.NoError(t, err)

		// Expire the key deterministically: the TTL running out is exactly a
		// delete from the caller's point of view.
		require.NoError(t, valkey.Del(ctx, "lock:"+key).Err())

		current, err := store.Acquire(ctx, key, lockTTL)
		require.NoError(t, err)

		require.ErrorIs(t, store.Release(ctx, stale), apperr.ErrLockNotHeld)

		_, err = store.Acquire(ctx, key, lockTTL)
		require.ErrorIs(t, err, apperr.ErrLockBusy, "the current owner's lock must survive a stale release")

		require.NoError(t, store.Release(ctx, current))
	})

	t.Run("release after expiry with nobody holding", func(t *testing.T) {
		t.Parallel()
		key := newKey()

		lock, err := store.Acquire(ctx, key, lockTTL)
		require.NoError(t, err)
		require.NoError(t, valkey.Del(ctx, "lock:"+key).Err())

		require.ErrorIs(t, store.Release(ctx, lock), apperr.ErrLockNotHeld)
	})

	t.Run("nil lock", func(t *testing.T) {
		t.Parallel()

		require.Error(t, store.Release(ctx, nil))
	})
}

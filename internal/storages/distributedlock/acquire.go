package distributedlock

import (
	"context"
	"errors"
	"fmt"
	"time"

	valkeylib "github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/apperr"
)

// Acquire tries to set a lock key with NX (only if not exists) and a TTL.
// Returns model.ErrLockBusy if the lock is already held by another caller.
func (l *Store) Acquire(ctx context.Context, key string, ttl time.Duration) error {
	res, err := l.client.SetArgs(ctx, lockPrefix+key, 1, valkeylib.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Result()

	if err != nil {
		if errors.Is(err, valkeylib.Nil) {
			return apperr.ErrLockBusy
		}
		return fmt.Errorf("acquire lock %s: %w", key, err)
	}

	if res != "OK" {
		return apperr.ErrLockBusy
	}
	return nil
}

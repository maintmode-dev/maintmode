package distributedlock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	valkeylib "github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Acquire tries to set a lock key with NX (only if not exists) and a TTL, under
// a fresh random owner token. Returns apperr.ErrLockBusy if the lock is already
// held by another caller.
func (l *Store) Acquire(ctx context.Context, key string, ttl time.Duration) (*entity.DistributedLock, error) {
	lock := &entity.DistributedLock{Key: lockPrefix + key, Token: uuid.NewString()}

	res, err := l.client.SetArgs(ctx, lock.Key, lock.Token, valkeylib.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Result()

	if err != nil {
		if errors.Is(err, valkeylib.Nil) {
			return nil, apperr.ErrLockBusy
		}
		return nil, fmt.Errorf("acquire lock %s: %w", key, err)
	}

	if res != "OK" {
		return nil, apperr.ErrLockBusy
	}
	return lock, nil
}

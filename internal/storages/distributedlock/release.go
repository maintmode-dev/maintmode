package distributedlock

import (
	"context"
	"errors"
	"fmt"

	valkeylib "github.com/redis/go-redis/v9"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// releaseScript deletes the key only while it still holds the caller's token.
// Compare and delete are one script, so a lock that expired and was taken by
// someone else between the two steps is never deleted.
var releaseScript = valkeylib.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

// Release deletes the lock key if the caller still owns it. Returns
// apperr.ErrLockNotHeld when the lock has expired, whoever holds the key now.
func (l *Store) Release(ctx context.Context, lock *entity.DistributedLock) error {
	if lock == nil {
		return errors.New("release lock: nil lock")
	}

	deleted, err := releaseScript.Run(ctx, l.client, []string{lock.Key}, lock.Token).Int()
	if err != nil {
		return fmt.Errorf("release lock %s: %w", lock.Key, err)
	}
	if deleted == 0 {
		return fmt.Errorf("release lock %s: %w", lock.Key, apperr.ErrLockNotHeld)
	}
	return nil
}

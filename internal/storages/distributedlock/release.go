package distributedlock

import (
	"context"
	"fmt"
)

// Release deletes the lock key.
func (l *Store) Release(ctx context.Context, key string) error {
	err := l.client.Del(ctx, lockPrefix+key).Err()
	if err != nil {
		return fmt.Errorf("release lock %s: %w", key, err)
	}
	return nil
}

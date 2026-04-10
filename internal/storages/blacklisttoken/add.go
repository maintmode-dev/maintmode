package blacklisttoken

import (
	"context"
	"fmt"
	"time"

	"github.com/ruko1202/xlog"
)

// Add places jti in the blacklist with TTL = remaining lifetime of the access token.
// If expiration <= 0, the token is already expired and no entry is created.
func (s *Store) Add(ctx context.Context, jti string, expiration time.Duration) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.tokenBlacklist.Add")
	defer span.End()

	if expiration <= 0 {
		return nil
	}

	err := s.db.Set(ctx, keyPrefix+jti, 1, expiration).Err()
	if err != nil {
		return fmt.Errorf("blacklist add %s: %w", jti, err)
	}
	return nil
}

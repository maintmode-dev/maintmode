package blacklisttoken

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
)

// Contains checks whether jti is in the blacklist.
func (s *Store) Contains(ctx context.Context, jti string) (bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.tokenBlacklist.Contains")
	defer span.End()

	n, err := s.db.Exists(ctx, keyPrefix+jti).Result()
	if err != nil {
		return false, fmt.Errorf("blacklist check %s: %w", jti, err)
	}
	return n > 0, nil
}

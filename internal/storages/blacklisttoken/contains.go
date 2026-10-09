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

// IsRevoked reports whether an access token is revoked, either on its own (jti,
// blacklisted by logout) or through its session (sid, revoked by logout,
// logout-all, reuse detection, a password change or a block). Both are checked
// in a single EXISTS. An empty sessionID -- a token minted before the claim
// existed -- is checked by jti alone.
func (s *Store) IsRevoked(ctx context.Context, jti, sessionID string) (bool, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "store.tokenBlacklist.IsRevoked")
	defer span.End()

	keys := []string{keyPrefix + jti}
	if sessionID != "" {
		keys = append(keys, sessionKeyPrefix+sessionID)
	}

	n, err := s.db.Exists(ctx, keys...).Result()
	if err != nil {
		return false, fmt.Errorf("blacklist check %s: %w", jti, err)
	}
	return n > 0, nil
}

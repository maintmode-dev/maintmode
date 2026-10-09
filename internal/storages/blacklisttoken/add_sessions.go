package blacklisttoken

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	valkeylib "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"
)

// AddSessions marks sessions (refresh-token families) as revoked for
// expiration, which must cover the lifetime of any access token still carrying
// one of them as its sid. One round trip for all of them.
func (s *Store) AddSessions(ctx context.Context, expiration time.Duration, sessionIDs ...uuid.UUID) error {
	ctx, span := xlog.WithOperationSpan(ctx, "store.tokenBlacklist.AddSessions")
	defer span.End()

	if expiration <= 0 || len(sessionIDs) == 0 {
		return nil
	}

	_, err := s.db.Pipelined(ctx, func(pipe valkeylib.Pipeliner) error {
		for _, id := range sessionIDs {
			pipe.Set(ctx, sessionKeyPrefix+id.String(), 1, expiration)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("blacklist add sessions: %w", err)
	}
	return nil
}

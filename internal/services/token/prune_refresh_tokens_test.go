package token

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// PruneRefreshTokens drives a table-wide DELETE, so these tests run
// sequentially (no t.Parallel) and assert only on rows they created. The
// retention they pass keeps the cutoff a day in the past: a sweep reaching now
// would delete rows other DB-backed suites are using under `-p 2`.

func seedRefreshToken(
	ctx context.Context, t *testing.T, srv *Service, userID uuid.UUID, expiresAt time.Time, revoked bool,
) string {
	t.Helper()

	hash := uuid.NewString()
	require.NoError(t, srv.SaveRefreshToken(ctx, &entity.RefreshToken{
		Token:            hash,
		UserID:           userID,
		Family:           uuid.New(),
		ExpiresAt:        expiresAt,
		Revoked:          revoked,
		SessionStartedAt: expiresAt.Add(-30 * 24 * time.Hour),
	}))

	return hash
}

func refreshTokenExists(ctx context.Context, t *testing.T, srv *Service, hash string) bool {
	t.Helper()

	_, err := srv.tokensStore.GetByTokenHash(ctx, hash)
	if err != nil {
		require.ErrorIs(t, err, apperr.ErrRefreshTokenNotFound)
		return false
	}

	return true
}

// TestPruneRefreshTokens_DrainsExpiredKeepsTheRest is the sweep end to end:
// more expired rows than one batch holds are all removed in one call, and
// nothing that has not expired -- revoked or not -- is touched.
func TestPruneRefreshTokens_DrainsExpiredKeepsTheRest(t *testing.T) {
	ctx := context.Background()
	srv := initService(t)
	user := testdbutils.MakeUser(ctx, t, users.NewStore(db))
	now := xtime.UTCNow()

	expired := make([]string, 0, 6)
	for range 5 {
		expired = append(expired, seedRefreshToken(ctx, t, srv, user.ID, now.Add(-72*time.Hour), false))
	}
	expired = append(expired, seedRefreshToken(ctx, t, srv, user.ID, now.Add(-72*time.Hour), true))

	live := seedRefreshToken(ctx, t, srv, user.ID, now.Add(time.Hour), false)
	liveRevoked := seedRefreshToken(ctx, t, srv, user.ID, now.Add(time.Hour), true)
	insideRetention := seedRefreshToken(ctx, t, srv, user.ID, now.Add(-time.Hour), false)

	// batchLimit 2 forces several batches.
	require.NoError(t, srv.PruneRefreshTokens(ctx, 24*time.Hour, 2))

	for _, hash := range expired {
		require.False(t, refreshTokenExists(ctx, t, srv, hash), "every expired row must be drained")
	}
	require.True(t, refreshTokenExists(ctx, t, srv, live), "a live row must survive")
	require.True(t, refreshTokenExists(ctx, t, srv, liveRevoked),
		"a revoked row within its lifetime must survive: reuse detection reads it")
	require.True(t, refreshTokenExists(ctx, t, srv, insideRetention),
		"a row expired for less than the retention must survive")
}

// TestPruneRefreshTokens_NonPositiveRetentionFallsBack is the safety coercion:
// a zero or negative retention must become the default rather than a cutoff at
// or after now, which would delete sessions still in use.
func TestPruneRefreshTokens_NonPositiveRetentionFallsBack(t *testing.T) {
	ctx := context.Background()
	srv := initService(t)
	user := testdbutils.MakeUser(ctx, t, users.NewStore(db))
	now := xtime.UTCNow()

	live := seedRefreshToken(ctx, t, srv, user.ID, now.Add(time.Hour), false)
	recentlyExpired := seedRefreshToken(ctx, t, srv, user.ID, now.Add(-time.Hour), false)

	for _, retention := range []time.Duration{0, -24 * time.Hour} {
		require.NoError(t, srv.PruneRefreshTokens(ctx, retention, 0))
		require.True(t, refreshTokenExists(ctx, t, srv, live), "retention %s reached a live row", retention)
		require.True(t, refreshTokenExists(ctx, t, srv, recentlyExpired),
			"retention %s must fall back to the default, not to a cutoff at now", retention)
	}
}

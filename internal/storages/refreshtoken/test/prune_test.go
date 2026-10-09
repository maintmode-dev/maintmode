package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// Every cutoff below stays in the PAST. The DELETE is bounded by expires_at and
// limit alone -- never by user -- so a cutoff at or after now would reach the
// live rows other DB-backed suites create in this shared database under
// `-p 2`. Assertions look rows up by hash, so they are not disturbed by what
// else is in the table.

func saveTokenExpiringAt(
	ctx context.Context, t *testing.T, store *refreshtoken.Store, userID, family uuid.UUID, expiresAt time.Time, revoked bool,
) *entity.RefreshToken {
	t.Helper()

	rt := &entity.RefreshToken{
		Token:            uuid.NewString(),
		UserID:           userID,
		Family:           family,
		ExpiresAt:        expiresAt,
		Revoked:          revoked,
		SessionStartedAt: expiresAt.Add(-30 * 24 * time.Hour),
	}
	require.NoError(t, store.Save(ctx, rt))

	return rt
}

func tokenExists(ctx context.Context, t *testing.T, store *refreshtoken.Store, hash string) bool {
	t.Helper()

	_, err := store.GetByTokenHash(ctx, hash)
	if err != nil {
		require.ErrorIs(t, err, apperr.ErrRefreshTokenNotFound)
		return false
	}

	return true
}

// TestPruneExpiredBefore_DeletesOnlyRowsPastCutoff covers the age boundary,
// for revoked rows as well as live ones: a revoked row that has not expired is
// what reuse detection reads, so revocation alone must not make a row eligible.
func TestPruneExpiredBefore_DeletesOnlyRowsPastCutoff(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := refreshtoken.NewStore(db)
	now := xtime.UTCNow()
	cutoff := now.Add(-24 * time.Hour)

	user := testdbutils.MakeUser(ctx, t, userStore)
	family := uuid.New()

	expiredLive := saveTokenExpiringAt(ctx, t, store, user.ID, family, now.Add(-72*time.Hour), false)
	expiredRevoked := saveTokenExpiringAt(ctx, t, store, user.ID, family, now.Add(-48*time.Hour), true)
	// Past its expiry but inside the margin: kept until the cutoff reaches it.
	insideMargin := saveTokenExpiringAt(ctx, t, store, user.ID, family, now.Add(-time.Hour), false)
	liveRevoked := saveTokenExpiringAt(ctx, t, store, user.ID, uuid.New(), now.Add(time.Hour), true)
	live := saveTokenExpiringAt(ctx, t, store, user.ID, uuid.New(), now.Add(time.Hour), false)

	deleted, err := store.PruneExpiredBefore(ctx, cutoff, 1000)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(2))

	require.False(t, tokenExists(ctx, t, store, expiredLive.Token), "an expired row must be deleted")
	require.False(t, tokenExists(ctx, t, store, expiredRevoked.Token), "an expired revoked row must be deleted")
	require.True(t, tokenExists(ctx, t, store, insideMargin.Token), "a row inside the margin must survive")
	require.True(t, tokenExists(ctx, t, store, liveRevoked.Token), "a revoked row that has not expired must survive")
	require.True(t, tokenExists(ctx, t, store, live.Token), "a live row must survive")
}

// TestPruneExpiredBefore_RespectsLimit pins the batch bound: one call removes
// at most limit rows, oldest first.
//
// Not parallel, so the sweep in the test above cannot run in the middle of it.
// Another package's sweep still can, which is why every assertion here holds
// whatever else deletes these rows: a count above the limit, or the newer row
// gone while the older survives, is wrong under any interleaving.
func TestPruneExpiredBefore_RespectsLimit(t *testing.T) {
	ctx := context.Background()
	store := refreshtoken.NewStore(db)
	user := testdbutils.MakeUser(ctx, t, userStore)

	// Far enough in the past that this test's rows are the oldest in the
	// table, so "oldest first" picks between exactly these two.
	base := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	older := saveTokenExpiringAt(ctx, t, store, user.ID, uuid.New(), base, false)
	newer := saveTokenExpiringAt(ctx, t, store, user.ID, uuid.New(), base.Add(time.Hour), false)

	deleted, err := store.PruneExpiredBefore(ctx, base.Add(2*time.Hour), 1)
	require.NoError(t, err)
	require.LessOrEqual(t, deleted, int64(1), "one batch must not delete more than its limit")

	if !tokenExists(ctx, t, store, newer.Token) {
		require.False(t, tokenExists(ctx, t, store, older.Token), "the batch must take the oldest row first")
	}

	_, err = store.PruneExpiredBefore(ctx, base.Add(2*time.Hour), 10)
	require.NoError(t, err)
	require.False(t, tokenExists(ctx, t, store, older.Token))
	require.False(t, tokenExists(ctx, t, store, newer.Token), "the next batch drains the rest")
}

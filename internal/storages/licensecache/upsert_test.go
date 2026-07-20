package licensecache

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
)

var errRollbackLicenseCache = errors.New("rollback license-cache test tx")

// The table is a singleton on a shared dev DB, so the empty-cache, first-write
// and overwrite paths run sequentially inside one rolled-back REPEATABLE READ
// transaction: the license integration test (services/license) touches the
// same row from another package, and tx isolation keeps them from clobbering
// each other under the parallel package runner.
func TestUpsertGet_SingletonLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	err := dbtx.NewTxManager(db).WithinTx(ctx, func(ctx context.Context) error {
		// Reset to the empty state regardless of what previous runs left behind.
		// Executor(ctx) resolves to the surrounding tx — a raw db.ExecContext
		// would bypass it and commit the DELETE for real.
		_, err := store.db.Executor(ctx).ExecContext(ctx, "DELETE FROM license_cache")
		require.NoError(t, err)

		_, err = store.Get(ctx)
		require.ErrorIs(t, err, apperr.ErrLicenseCacheEmpty)

		// First successful heartbeat creates the row.
		first := &entity.License{
			Status:         entity.LicenseStatusBlocked,
			SeatsPurchased: lo.ToPtr(int64(5)),
			FetchedAt:      lo.ToPtr(time.Now().UTC().Truncate(time.Microsecond)),
		}
		require.NoError(t, store.Upsert(ctx, first))

		got, err := store.Get(ctx)
		require.NoError(t, err)
		require.Equal(t, entity.LicenseStatusBlocked, got.Status)
		require.EqualValues(t, 5, *got.SeatsPurchased)
		require.True(t, got.FetchedAt.Equal(*first.FetchedAt), "fetched_at: got %v want %v", got.FetchedAt, first.FetchedAt)

		// Every later heartbeat overwrites the Console-owned fields in place.
		second := &entity.License{
			Status:         entity.LicenseStatusActive,
			SeatsPurchased: lo.ToPtr(int64(10)),
			FetchedAt:      lo.ToPtr(time.Now().UTC().Truncate(time.Microsecond)),
		}
		require.NoError(t, store.Upsert(ctx, second))

		got, err = store.Get(ctx)
		require.NoError(t, err)
		require.Equal(t, entity.LicenseStatusActive, got.Status)
		require.EqualValues(t, 10, *got.SeatsPurchased)
		require.True(t, got.FetchedAt.Equal(*second.FetchedAt), "fetched_at: got %v want %v", got.FetchedAt, second.FetchedAt)

		// Still exactly one row.
		var count int
		require.NoError(t, store.db.Executor(ctx).QueryRowxContext(ctx, "SELECT COUNT(*) FROM license_cache").Scan(&count))
		require.Equal(t, 1, count)

		return errRollbackLicenseCache
	}, dbtx.WithIsolation(sql.LevelRepeatableRead))
	require.ErrorIs(t, err, errRollbackLicenseCache)
}

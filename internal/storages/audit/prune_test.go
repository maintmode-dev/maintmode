package audit

import (
	"context"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// countByMarker returns how many audit rows carry the given action marker. The
// shared DB holds rows from other tests/runs, so prune assertions scope to a
// per-test marker rather than the global table.
func countByMarker(ctx context.Context, t *testing.T, marker string) int64 {
	t.Helper()

	var dest struct {
		Count int64 `alias:"c"`
	}
	stmt := table.AuditLog.
		SELECT(postgres.COUNT(postgres.STAR).AS("c")).
		WHERE(table.AuditLog.Action.EQ(postgres.String(marker)))
	require.NoError(t, stmt.QueryContext(ctx, db, &dest))

	return dest.Count
}

// PruneOlderThan is a global, destructive DELETE on a shared table, so these
// tests must not run in parallel with each other: a sibling's large-limit prune
// would delete this test's backdated rows out from under it. They run sequentially
// (no t.Parallel) and scope every assertion to a per-run marker. Rows other tests
// write via AddLog are stamped NOW(), so no realistic cutoff here ever deletes
// them — only these tests create backdated (prunable) rows.

// TestPruneOlderThan_DeletesOnlyExpired inserts a mix of old and fresh rows under
// a unique marker, prunes with a generous limit, and asserts only the expired
// rows for this marker are gone.
func TestPruneOlderThan_DeletesOnlyExpired(t *testing.T) {
	ctx := context.Background()
	store := NewStore(db)
	marker := "prune-expired-" + xuuid.NewString()
	now := xtime.UTCNow()

	// 3 expired (older than the 30-day cutoff), 2 fresh.
	for i := range 3 {
		insertLogAt(ctx, t, marker, now.Add(-time.Duration(40+i)*24*time.Hour))
	}
	insertLogAt(ctx, t, marker, now.Add(-time.Hour))
	insertLogAt(ctx, t, marker, now)

	require.Equal(t, int64(5), countByMarker(ctx, t, marker))

	cutoff := now.Add(-30 * 24 * time.Hour)
	// Limit is large enough to drain every expired row in one call.
	deleted, err := store.PruneOlderThan(ctx, cutoff, 1_000_000)
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(3), "must delete at least this marker's 3 expired rows")

	require.Equal(t, int64(2), countByMarker(ctx, t, marker), "only the 2 fresh rows of this marker remain")
}

// TestPruneOlderThan_RespectsLimit asserts one batch never deletes more than the
// limit, even with more expired rows available.
func TestPruneOlderThan_RespectsLimit(t *testing.T) {
	ctx := context.Background()
	store := NewStore(db)
	marker := "prune-limit-" + xuuid.NewString()
	now := xtime.UTCNow()

	// More expired rows than the batch limit. The shared DB may hold other expired
	// rows too, so we assert the batch size cap (deleted == limit) — which holds
	// regardless of which rows are chosen — rather than which specific rows survive.
	const total = 5
	for i := range total {
		insertLogAt(ctx, t, marker, now.AddDate(-10, 0, -i))
	}

	cutoff := now.Add(-24 * time.Hour)
	deleted, err := store.PruneOlderThan(ctx, cutoff, 2)
	require.NoError(t, err)
	require.Equal(t, int64(2), deleted, "a single batch deletes exactly the limit when more rows are expired")
}

// TestPruneOlderThan_NothingExpired returns zero when no row predates the cutoff.
func TestPruneOlderThan_NothingExpired(t *testing.T) {
	ctx := context.Background()
	store := NewStore(db)
	marker := "prune-none-" + xuuid.NewString()
	now := xtime.UTCNow()

	insertLogAt(ctx, t, marker, now)
	insertLogAt(ctx, t, marker, now.Add(-time.Hour))

	// Cutoff older than any realistic row (year 2000): a true no-op. Using a fixed
	// far-past instant keeps this from deleting other tests' backdated rows.
	deleted, err := store.PruneOlderThan(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), 1_000_000)
	require.NoError(t, err)
	require.Equal(t, int64(0), deleted, "no row predates a year-2000 cutoff")

	require.Equal(t, int64(2), countByMarker(ctx, t, marker), "this marker's rows are untouched")
}

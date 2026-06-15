package auditor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// Prune drives a global, destructive DELETE, so these tests run sequentially (no
// t.Parallel) and scope assertions to a per-run marker — see the note in the
// storage-layer prune_test.go.

// TestPrune_DrainsAcrossBatches inserts more expired rows than one batch holds and
// asserts the drain loop clears them all in a single Prune call.
func TestPrune_DrainsAcrossBatches(t *testing.T) {
	ctx := context.Background()
	auditor := NewAuditor(store)
	marker := "svc-prune-drain-" + xuuid.NewString()
	now := xtime.UTCNow()

	// 5 expired rows (~1 year old) + 1 fresh. batchLimit 2 forces several batches.
	for range 5 {
		insertLogAt(ctx, t, marker, now.AddDate(-1, 0, 0))
	}
	insertLogAt(ctx, t, marker, now)
	require.Equal(t, int64(6), countByMarker(ctx, t, marker))

	// retention 30d → cutoff is 30 days ago; the 1-year-old rows are all expired.
	err := auditor.Prune(ctx, 30*24*time.Hour, 2)
	require.NoError(t, err)

	require.Equal(t, int64(1), countByMarker(ctx, t, marker),
		"drain loop must clear every expired row across batches, leaving only the fresh one")
}

// TestPrune_NonPositiveBatchLimitStillDrains guards the loop-termination fix: a
// non-positive batchLimit must fall back to the default rather than spinning all
// batches deleting nothing. We assert the expired rows are actually removed.
func TestPrune_NonPositiveBatchLimitStillDrains(t *testing.T) {
	ctx := context.Background()
	auditor := NewAuditor(store)
	marker := "svc-prune-badlimit-" + xuuid.NewString()
	now := xtime.UTCNow()

	for range 3 {
		insertLogAt(ctx, t, marker, now.AddDate(-1, 0, 0))
	}
	insertLogAt(ctx, t, marker, now)
	require.Equal(t, int64(4), countByMarker(ctx, t, marker))

	err := auditor.Prune(ctx, 30*24*time.Hour, 0)
	require.NoError(t, err)

	require.Equal(t, int64(1), countByMarker(ctx, t, marker),
		"batchLimit 0 must fall back to the default and still drain expired rows")
}

// TestPrune_KeepsRowsWithinRetention is a no-op when all rows are inside the
// window.
func TestPrune_KeepsRowsWithinRetention(t *testing.T) {
	ctx := context.Background()
	auditor := NewAuditor(store)
	marker := "svc-prune-keep-" + xuuid.NewString()
	now := xtime.UTCNow()

	insertLogAt(ctx, t, marker, now)
	insertLogAt(ctx, t, marker, now.Add(-24*time.Hour))
	require.Equal(t, int64(2), countByMarker(ctx, t, marker))

	// 1-year retention: both rows are well within the window.
	err := auditor.Prune(ctx, 365*24*time.Hour, 100)
	require.NoError(t, err)

	require.Equal(t, int64(2), countByMarker(ctx, t, marker), "rows within retention are kept")
}

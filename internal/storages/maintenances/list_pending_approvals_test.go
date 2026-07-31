package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// The test DB is shared and tests run in parallel, so every subtest invents its
// own approver id. Filtering on a freshly generated approver yields a fully
// private slice of rows, which is what makes the assertions on total, ordering
// and offsets stable under -count 2 and t.Parallel().
func TestStore_ListPendingApprovals(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	t.Run("draft maintenance assigned to the approver appears", func(t *testing.T) {
		t.Parallel()
		approverID := xuuid.New()

		maint := makePendingMaint(ctx, t, store, approverID)

		page, total, err := store.ListPendingApprovals(ctx, filterFor(approverID))
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, page, 1)
		require.Equal(t, maint.ID, page[0].ID)
		require.Equal(t, entity.MaintenanceStatusDraft, page[0].Status)
	})

	t.Run("maintenance assigned to another approver never appears", func(t *testing.T) {
		t.Parallel()
		me, other := xuuid.New(), xuuid.New()

		makePendingMaint(ctx, t, store, other)

		page, total, err := store.ListPendingApprovals(ctx, filterFor(me))
		require.NoError(t, err)
		require.EqualValues(t, 0, total)
		require.Empty(t, page)
	})

	t.Run("non-draft statuses never appear", func(t *testing.T) {
		t.Parallel()

		for _, status := range []entity.MaintenanceStatus{
			entity.MaintenanceStatusPlanned,
			entity.MaintenanceStatusInProgress,
			entity.MaintenanceStatusCompleted,
			entity.MaintenanceStatusCancelled,
		} {
			t.Run(string(status), func(t *testing.T) {
				t.Parallel()
				approverID := xuuid.New()

				makeMaintForApprover(ctx, t, store, approverID, status)

				page, total, err := store.ListPendingApprovals(ctx, filterFor(approverID))
				require.NoError(t, err)
				require.EqualValues(t, 0, total, "status %s must not be pending approval", status)
				require.Empty(t, page)
			})
		}
	})

	t.Run("total counts only rows matching the filter", func(t *testing.T) {
		t.Parallel()
		approverID := xuuid.New()

		// Two of ours plus noise that must not be counted: another approver's
		// draft and our own already-approved maintenance.
		makePendingMaint(ctx, t, store, approverID)
		makePendingMaint(ctx, t, store, approverID)
		makePendingMaint(ctx, t, store, xuuid.New())
		makeMaintForApprover(ctx, t, store, approverID, entity.MaintenanceStatusPlanned)

		page, total, err := store.ListPendingApprovals(ctx, filterFor(approverID))
		require.NoError(t, err)
		require.EqualValues(t, 2, total)
		require.Len(t, page, 2)
	})

	t.Run("limit and offset cut the page while total stays whole", func(t *testing.T) {
		t.Parallel()
		approverID := xuuid.New()

		makePendingMaint(ctx, t, store, approverID)
		makePendingMaint(ctx, t, store, approverID)

		full, total, err := store.ListPendingApprovals(ctx, filterFor(approverID))
		require.NoError(t, err)
		require.EqualValues(t, 2, total)
		require.Len(t, full, 2)

		first, total, err := store.ListPendingApprovals(ctx, pageFor(approverID, 1, 0))
		require.NoError(t, err)
		require.EqualValues(t, 2, total, "total must ignore limit/offset")
		require.Len(t, first, 1)
		require.Equal(t, full[0].ID, first[0].ID)

		second, _, err := store.ListPendingApprovals(ctx, pageFor(approverID, 1, 1))
		require.NoError(t, err)
		require.Len(t, second, 1)
		require.Equal(t, full[1].ID, second[0].ID)
	})

	// Ordering rests on insertion order: created_at cannot be pinned from the
	// fixture because CreateMaint inserts MutableColumns.Except(CreatedAt), so the
	// DB stamps DEFAULT now() and any CreatedAt set in MakeMaint is discarded.
	// now() has microsecond resolution and advances between transactions, so
	// insertion order is observable.
	//
	// The id tie-break in ORDER BY is NOT covered by this test and cannot be:
	// proving it needs two rows with bit-identical created_at, which inserts
	// cannot produce. It stays in the query to keep pagination stable and is
	// verified by reading the code only — do not read this test as covering it.
	t.Run("oldest first, page monotonic by created_at", func(t *testing.T) {
		t.Parallel()
		approverID := xuuid.New()

		oldest := makePendingMaint(ctx, t, store, approverID)
		newest := makePendingMaint(ctx, t, store, approverID)

		page, _, err := store.ListPendingApprovals(ctx, filterFor(approverID))
		require.NoError(t, err)
		require.Len(t, page, 2)

		require.Equal(t, oldest.ID, page[0].ID, "the older maintenance must come first")
		require.Equal(t, newest.ID, page[1].ID)

		for i := 1; i < len(page); i++ {
			prev, cur := page[i-1], page[i]
			require.Falsef(t, cur.CreatedAt.Before(prev.CreatedAt),
				"page not sorted by created_at asc at index %d", i)
		}
	})

	t.Run("updated_at is null until the maintenance is updated", func(t *testing.T) {
		t.Parallel()
		approverID := xuuid.New()

		maint := makePendingMaint(ctx, t, store, approverID)

		page, _, err := store.ListPendingApprovals(ctx, filterFor(approverID))
		require.NoError(t, err)
		require.Len(t, page, 1)
		require.Nil(t, page[0].UpdatedAt, "an untouched maintenance carries no updated_at")

		maint.Title = "Updated" + xuuid.NewString()
		require.NoError(t, store.UpdateMaint(ctx, maint))

		page, _, err = store.ListPendingApprovals(ctx, filterFor(approverID))
		require.NoError(t, err)
		require.Len(t, page, 1)
		require.NotNil(t, page[0].UpdatedAt)
	})

	t.Run("nil approver is an error, not an unfiltered listing", func(t *testing.T) {
		t.Parallel()

		page, total, err := store.ListPendingApprovals(ctx, filterFor(uuid.Nil))
		require.ErrorIs(t, err, apperr.ErrNilApprover)
		require.Empty(t, page, "a nil approver must never leak other users' drafts")
		require.Zero(t, total)
	})
}

// filterFor asks for a page big enough to hold everything a subtest seeds, so
// assertions on total and ordering read the whole private slice at once.
func filterFor(approverID uuid.UUID) *calendardto.PendingApprovalsFilter {
	return pageFor(approverID, 50, 0)
}

func pageFor(approverID uuid.UUID, limit, offset int64) *calendardto.PendingApprovalsFilter {
	return &calendardto.PendingApprovalsFilter{
		ApproverUserID: approverID,
		Limit:          limit,
		Offset:         offset,
	}
}

// makePendingMaint creates a draft maintenance awaiting the given approver —
// the only shape this listing is supposed to return.
func makePendingMaint(ctx context.Context, t *testing.T, store *Store, approverID uuid.UUID) *entity.Maintenance {
	t.Helper()

	return makeMaintForApprover(ctx, t, store, approverID, entity.MaintenanceStatusDraft)
}

// makeMaintForApprover is the local fixture for this listing: the shared
// testdbutils.MakeMaint cannot be used here because that package imports this
// store, and importing it back would be a test-only import cycle. The package's
// own makeMaint fixes status to planned and randomizes the approver, so neither
// axis this query filters on is reachable through it.
func makeMaintForApprover(
	ctx context.Context,
	t *testing.T,
	store *Store,
	approverID uuid.UUID,
	status entity.MaintenanceStatus,
) *entity.Maintenance {
	t.Helper()

	created, err := store.CreateMaint(ctx, &entity.Maintenance{
		Title:          "Title" + xuuid.NewString(),
		Description:    "Description" + t.Name(),
		PlannedPeriod:  somePeriod(),
		Scope:          entity.MaintenanceScopeResources,
		Status:         status,
		Impact:         entity.MaintenanceImpactFull,
		ApproverUserID: approverID,
	})
	require.NoError(t, err)

	return created
}

func somePeriod() entity.Period {
	start := xtime.UTCNow().Add(24 * time.Hour)
	return entity.NewPeriod(start, start.Add(2*time.Hour))
}

package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

func TestGetActualConflicts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := initService(db)

	t.Run("reports the neighbor's own resources, not the shared subset", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)
		neighborOwn := testdbutils.MakeResource(ctx, t, resourcesStore)

		maint := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
			testdbutils.WithActualPeriod(entity.NewPeriod(start, end)),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		neighborPeriod := entity.NewPeriod(start.Add(time.Hour), end)
		neighbor := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			neighborPeriod,
			testdbutils.WithStatus(entity.MaintenanceStatusCancelled),
			testdbutils.WithActualPeriod(neighborPeriod),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID, neighborOwn.ID),
		)

		conflicts, truncated, err := s.GetActualConflicts(ctx, &entity.ActualConflictQueryCmd{
			MaintID:      maint.ID,
			ActualPeriod: lo.FromPtr(maint.ActualPeriod),
			Scope:        maint.Scope,
			ResourceIDs:  maint.Resources,
		})
		require.NoError(t, err)
		require.False(t, truncated)

		actual, found := lo.Find(conflicts, func(c *entity.ConflictWithResources) bool {
			return c.MaintenanceID == neighbor.ID
		})
		require.True(t, found)
		require.ElementsMatch(t, []uuid.UUID{shared.ID, neighborOwn.ID}, actual.Resources,
			"a conflict reports what its own maintenance touches, not the intersection with ours")
	})
}

// AC7. The snapshot branch must re-resolve each neighbor's resources from the
// live tables rather than trusting the stored rows, which arrive verbatim from
// the approve request body and are not covered by the approve fingerprint.
//
// The fixture makes the two disagree on purpose: the stored snapshot names one
// resource, the neighbor actually holds another. Only re-resolution can report
// the live one, so the test fails on an implementation that reads the rows.
func TestGetApprovalSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := initService(db)
	snapshotStore := conflictsnapshots.NewStore(db)

	t.Run("re-resolves resources from the live tables", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		stale := testdbutils.MakeResource(ctx, t, resourcesStore)
		live := testdbutils.MakeResource(ctx, t, resourcesStore)

		subject := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithStatus(entity.MaintenanceStatusCancelled),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(live.ID),
		)
		neighbor := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(live.ID),
		)

		// Stored snapshot deliberately disagrees with what the neighbor holds.
		require.NoError(t, snapshotStore.Save(ctx, subject.ID, []*entity.ConflictWithResources{
			{
				Conflict: &entity.Conflict{
					MaintenanceID: neighbor.ID,
					Title:         neighbor.Title,
					Scope:         neighbor.Scope,
					OverlapStart:  start,
					OverlapEnd:    end,
				},
				Resources: []uuid.UUID{stale.ID},
			},
		}))

		snapshot, err := s.GetApprovalSnapshot(ctx, subject.ID)
		require.NoError(t, err)
		require.Len(t, snapshot, 1)

		require.True(t, snapshot[0].KnownAtApproval,
			"everything on this branch was seen at approval by construction")
		require.Equal(t, []uuid.UUID{live.ID}, snapshot[0].Resources,
			"resources come from the live tables, not the client-supplied snapshot rows")
	})

	// GetSnapshots has no ORDER BY, and every row on this branch is flagged
	// known, so SortConflicts' overlap-start tiebreak is the only thing keeping
	// the card stable between reloads. With one neighbor in the fixture that is
	// unobservable, so this needs two.
	t.Run("orders snapshot entries by overlap start", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		subject := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithStatus(entity.MaintenanceStatusCancelled),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)

		neighbors := lo.Times(2, func(_ int) *entity.Maintenance {
			return testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
				entity.NewPeriod(start, end),
				testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
				testdbutils.WithScope(entity.MaintenanceScopeResources),
				testdbutils.WithResources(shared.ID),
			)
		})

		// Saved later-first, so a stable result cannot come from insertion order.
		require.NoError(t, snapshotStore.Save(ctx, subject.ID, []*entity.ConflictWithResources{
			snapshotEntry(neighbors[0], start.Add(2*time.Hour), end, shared.ID),
			snapshotEntry(neighbors[1], start.Add(time.Hour), end, shared.ID),
		}))

		snapshot, err := s.GetApprovalSnapshot(ctx, subject.ID)
		require.NoError(t, err)
		require.Len(t, snapshot, 2)

		require.Equal(t, neighbors[1].ID, snapshot[0].MaintenanceID,
			"the earlier overlap leads regardless of how the rows were stored")
		require.Equal(t, neighbors[0].ID, snapshot[1].MaintenanceID)
	})

	t.Run("a maintenance that was never approved reports nothing", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		subject := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithStatus(entity.MaintenanceStatusCancelled),
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
		)

		snapshot, err := s.GetApprovalSnapshot(ctx, subject.ID)
		require.NoError(t, err)
		require.Empty(t, snapshot, "a draft canceled before approval has no snapshot to show")
	})
}

// snapshotEntry builds one frozen conflict row as the approve path would.
func snapshotEntry(
	neighbor *entity.Maintenance,
	overlapStart, overlapEnd time.Time,
	resourceID uuid.UUID,
) *entity.ConflictWithResources {
	return &entity.ConflictWithResources{
		Conflict: &entity.Conflict{
			MaintenanceID: neighbor.ID,
			Title:         neighbor.Title,
			Scope:         neighbor.Scope,
			OverlapStart:  overlapStart,
			OverlapEnd:    overlapEnd,
		},
		Resources: []uuid.UUID{resourceID},
	}
}

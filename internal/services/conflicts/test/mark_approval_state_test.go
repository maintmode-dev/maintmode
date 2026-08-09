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
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// TestMarkApprovalState_RoundTrip closes the one gap the mocked service tests
// cannot: it writes a snapshot with the real Save and reads it back through the
// real GetSnapshots, so a mapping regression between the two — a transposed
// maintenance_id / conflicted_maintenance_id, say — fails here instead of
// shipping. Every other test in this feature either mocks the read or exercises
// only one side of the pair.
//
// It approves nothing: the approve path recomputes conflicts under a
// serializable transaction and rejects the call if the set moved, which on a
// shared database it constantly does. Writing the snapshot directly reaches the
// same rows by the same code.
func TestMarkApprovalState_RoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	start, end := now.Add(time.Hour), now.Add(5*time.Hour)

	s := initService(db)
	// The same store the service reads through, so the test can seed a snapshot
	// without going through approve.
	snapshotStore := conflictsnapshots.NewStore(db)

	sharedResource := testdbutils.MakeResource(ctx, t, resourcesStore)

	subject := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
		entity.NewPeriod(start, end),
		testdbutils.WithScope(entity.MaintenanceScopeResources),
		testdbutils.WithResources(sharedResource.ID),
	)

	// Present when the subject was approved, so it goes into the snapshot.
	known := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
		entity.NewPeriod(start, end.Add(-time.Hour)),
		testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		testdbutils.WithScope(entity.MaintenanceScopeResources),
		testdbutils.WithResources(sharedResource.ID),
	)

	// Appeared afterwards: never in the snapshot, so nobody reviewed it.
	fresh := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
		entity.NewPeriod(start.Add(time.Hour), end),
		testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		testdbutils.WithScope(entity.MaintenanceScopeResources),
		testdbutils.WithResources(sharedResource.ID),
	)

	require.NoError(t, snapshotStore.Save(ctx, subject.ID, []*entity.ConflictWithResources{
		{
			Conflict: &entity.Conflict{
				MaintenanceID: known.ID,
				OverlapStart:  start,
				OverlapEnd:    end.Add(-time.Hour),
				Scope:         entity.MaintenanceScopeResources,
			},
			Resources: []uuid.UUID{sharedResource.ID},
		},
	}))

	live, err := s.GetConflicts(ctx, &entity.ConflictQueryCmd{
		MaintID:       subject.ID,
		PlannedPeriod: entity.NewPeriod(start, end),
		Scope:         entity.MaintenanceScopeResources,
		ResourceIDs:   []uuid.UUID{sharedResource.ID},
	})
	require.NoError(t, err)

	got := s.MarkApprovalState(ctx, subject.ID, live)

	knownView, found := lo.Find(got, func(c *entity.ConflictWithResources) bool {
		return c.MaintenanceID == known.ID
	})
	require.True(t, found, "the conflict written to the snapshot must still be listed")
	require.True(t, knownView.KnownAtApproval, "it was in the snapshot, so the approver saw it")

	freshView, found := lo.Find(got, func(c *entity.ConflictWithResources) bool {
		return c.MaintenanceID == fresh.ID
	})
	require.True(t, found, "the later conflict must be listed too")
	require.False(t, freshView.KnownAtApproval, "it was never in the snapshot")

	require.Less(t,
		lo.IndexOf(got, freshView),
		lo.IndexOf(got, knownView),
		"unreviewed conflicts lead",
	)
}

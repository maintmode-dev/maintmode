package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testtimeutils "github.com/ruko1202/maintmode/test/utils/time"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// The factual query answers a different question than ConflictedMaints: not
// "who may collide with my plan" but "who was actually working while I was".
// The load-bearing difference is the absence of a status filter — a maintenance
// that ran without ever being approved is invisible to the live query (wrong
// status) and to the approval snapshot (it was not there to be seen), so this
// query is the only place it can surface.
func TestActualConflictedMaints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := conflicts.NewStore(db)

	// ran builds a maintenance that actually worked over the given window.
	ran := func(
		t *testing.T,
		period entity.Period,
		status entity.MaintenanceStatus,
		changers ...testdbutils.MaintChanger,
	) *entity.Maintenance {
		t.Helper()
		return testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, period,
			append([]testdbutils.MaintChanger{
				testdbutils.WithStatus(status),
				testdbutils.WithActualPeriod(period),
			}, changers...)...,
		)
	}

	t.Run("a completed neighbor that ran alongside is reported", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		maint := ran(t, entity.NewPeriod(start, end), entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		neighborPeriod := entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour))
		neighbor := ran(t, neighborPeriod, entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)

		conflicted, truncated, err := store.ActualConflictedMaints(ctx, actualCmd(maint))
		require.NoError(t, err)
		require.False(t, truncated)

		actual, found := lo.Find(conflicted, func(c *entity.Conflict) bool {
			return c.MaintenanceID == neighbor.ID
		})
		require.True(t, found, "a neighbor that ran in our window must be reported")

		require.Equal(t, &entity.Conflict{
			MaintenanceID: neighbor.ID,
			Title:         neighbor.Title,
			Scope:         neighbor.Scope,
			OverlapStart:  testtimeutils.OverlapStart(lo.FromPtr(maint.ActualPeriod), neighborPeriod),
			OverlapEnd:    testtimeutils.OverlapEnd(lo.FromPtr(maint.ActualPeriod), neighborPeriod),
		}, actual, "the reported window is the intersection of the two actual periods")
	})

	// The whole point of the feature. The live query filters neighbors to
	// planned/in_progress, so a canceled one is invisible there; here its final
	// status is irrelevant because it demonstrably did work.
	t.Run("a canceled neighbor that had started is reported", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		maint := ran(t, entity.NewPeriod(start, end), entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		neighbor := ran(t, entity.NewPeriod(start.Add(time.Hour), end), entity.MaintenanceStatusCancelled,
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)

		conflicted, _, err := store.ActualConflictedMaints(ctx, actualCmd(maint))
		require.NoError(t, err)

		require.True(t,
			lo.ContainsBy(conflicted, func(c *entity.Conflict) bool {
				return c.MaintenanceID == neighbor.ID
			}),
			"cancellation does not undo work already done, so a canceled neighbor that ran must be reported")
	})

	t.Run("a neighbor that never started is not reported", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		maint := ran(t, entity.NewPeriod(start, end), entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		// Planned right over our window and sharing our resource — every reason
		// to conflict except the one that counts: it has no actual period.
		neighbor := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)

		conflicted, _, err := store.ActualConflictedMaints(ctx, actualCmd(maint))
		require.NoError(t, err)

		require.False(t,
			lo.ContainsBy(conflicted, func(c *entity.Conflict) bool {
				return c.MaintenanceID == neighbor.ID
			}),
			"a maintenance that never ran cannot have caused an incident")
	})

	t.Run("a neighbor that ran outside our window is not reported", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		maint := ran(t, entity.NewPeriod(start, end), entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		// Starts exactly where ours ended: half-open ranges do not touch.
		neighbor := ran(t, entity.NewPeriod(end, end.Add(time.Hour)), entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
		)

		conflicted, _, err := store.ActualConflictedMaints(ctx, actualCmd(maint))
		require.NoError(t, err)

		require.False(t,
			lo.ContainsBy(conflicted, func(c *entity.Conflict) bool {
				return c.MaintenanceID == neighbor.ID
			}),
			"work that finished when ours began did not run alongside it")
	})

	// AC6. The exact timestamp matters: an unclamped open-ended range arrives as
	// a nil upper bound, which lo.FromPtr turns into the zero time — so asserting
	// merely "not infinite" would pass on the corrupt value this clamp prevents.
	t.Run("a still-running neighbor is clamped to our window", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		maint := ran(t, entity.NewPeriod(start, end), entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		neighborStart := start.Add(time.Hour)
		neighbor := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
			entity.NewPeriod(neighborStart, end.Add(24*time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusInProgress),
			testdbutils.WithActualPeriod(entity.NewOpenEndedPeriod(neighborStart)),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)

		conflicted, _, err := store.ActualConflictedMaints(ctx, actualCmd(maint))
		require.NoError(t, err)

		actual, found := lo.Find(conflicted, func(c *entity.Conflict) bool {
			return c.MaintenanceID == neighbor.ID
		})
		require.True(t, found, "a neighbor still running inside our window must be reported")
		// Truncated, not rounded, to what timestamptz holds: Go carries
		// nanoseconds and the driver drops the sub-microsecond remainder on the
		// way in, so a value with a remainder of 500ns or more comes back BELOW
		// what Round would produce. Comparing the raw Go value passes only on a
		// clock that never yields sub-microsecond precision — which is why this
		// is green on macOS and red on the Linux runner.
		require.Equal(t, neighborStart.Truncate(time.Microsecond), actual.OverlapStart)
		require.Equal(t, end.Truncate(time.Microsecond), actual.OverlapEnd,
			"an open-ended neighbor's overlap ends where our own window ends")
	})

	// A maintenance overlaps its own window perfectly, so without the
	// self-exclusion predicate every terminal card would report a phantom
	// conflict with itself — and burn one of its 200 slots on it. The existing
	// assertions all ask "does the list contain the neighbor", a shape that is
	// blind to extra rows, so this needs asking the other way round.
	t.Run("a maintenance does not report itself", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		maint := ran(t, entity.NewPeriod(start, end), entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
		)

		conflicted, _, err := store.ActualConflictedMaints(ctx, actualCmd(maint))
		require.NoError(t, err)

		require.False(t,
			lo.ContainsBy(conflicted, func(c *entity.Conflict) bool {
				return c.MaintenanceID == maint.ID
			}),
			"a maintenance is not its own conflict")
	})

	// The mirror of the still-running case below: there the NEIGHBOR's end is
	// clamped, here its start. Every other fixture places the neighbor at or
	// after our start, so GREATEST never has to choose and the lower clamp goes
	// unexercised — a neighbor that began before us would report an overlap
	// starting before our maintenance existed.
	t.Run("a neighbor that started before us is clamped to our window", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		maint := ran(t, entity.NewPeriod(start, end), entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		neighborPeriod := entity.NewPeriod(start.Add(-3*time.Hour), end)
		neighbor := ran(t, neighborPeriod, entity.MaintenanceStatusCompleted,
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)

		conflicted, _, err := store.ActualConflictedMaints(ctx, actualCmd(maint))
		require.NoError(t, err)

		actual, found := lo.Find(conflicted, func(c *entity.Conflict) bool {
			return c.MaintenanceID == neighbor.ID
		})
		require.True(t, found)
		require.Equal(t, start.Truncate(time.Microsecond), actual.OverlapStart,
			"the overlap cannot begin before our own window did")
	})

	t.Run("an open actual period is rejected", func(t *testing.T) {
		t.Parallel()

		start, _ := testdbutils.IsolatedPeriodBounds(t)

		_, _, err := store.ActualConflictedMaints(ctx, &entity.ActualConflictQueryCmd{
			MaintID:      testdbutils.MakeResource(ctx, t, resourcesStore).ID,
			ActualPeriod: entity.NewOpenEndedPeriod(start),
			Scope:        entity.MaintenanceScopeGlobal,
		})
		require.Error(t, err,
			"an open period is an invariant violation here: the caller routes a "+
				"maintenance without a closed window to the snapshot branch instead")
	})
}

// AC9. Truncation must be detected rather than inferred from a full page, so the
// query fetches one row past the cap. Seeding 201 neighbors is expensive, hence
// one focused test rather than a table.
func TestActualConflictedMaintsTruncates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := conflicts.NewStore(db)
	start, end := testdbutils.IsolatedPeriodBounds(t)

	maint := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
		entity.NewPeriod(start, end),
		testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
		testdbutils.WithActualPeriod(entity.NewPeriod(start, end)),
		testdbutils.WithScope(entity.MaintenanceScopeGlobal),
	)

	for i := range conflicts.ActualConflictsLimit + 1 {
		// Global scope so every one of them conflicts on time alone, and a
		// distinct start so the ordering has something to work with.
		neighborStart := start.Add(time.Duration(i) * time.Minute)
		testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
			entity.NewPeriod(neighborStart, end),
			testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
			testdbutils.WithActualPeriod(entity.NewPeriod(neighborStart, end)),
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
		)
	}

	conflicted, truncated, err := store.ActualConflictedMaints(ctx, actualCmd(maint))
	require.NoError(t, err)
	require.True(t, truncated, "more neighbors than the cap must report truncation")
	require.Len(t, conflicted, conflicts.ActualConflictsLimit,
		fmt.Sprintf("the probe row past the cap must be dropped, leaving exactly %d",
			conflicts.ActualConflictsLimit))

	// WHICH rows survive matters as much as how many. Without an ORDER BY,
	// Postgres is free to return any 200 of them and the set would drift between
	// runs — so an incident reviewer would lose an arbitrary subset of the
	// neighbors with nothing indicating which. Asserting the length alone passes
	// happily on an unordered query.
	// Non-decreasing rather than strictly increasing: neighbors starting before
	// our window all clamp to the same start, so ties are expected and the id
	// tiebreak decides between them.
	require.IsNonDecreasingf(t,
		lo.Map(conflicted, func(c *entity.Conflict, _ int) time.Time { return c.OverlapStart }),
		"the reported neighbors must be ordered by overlap start, so truncation "+
			"drops the latest-starting ones deterministically")
}

func actualCmd(maint *entity.Maintenance) *entity.ActualConflictQueryCmd {
	return &entity.ActualConflictQueryCmd{
		MaintID:      maint.ID,
		ActualPeriod: lo.FromPtr(maint.ActualPeriod),
		Scope:        maint.Scope,
		ResourceIDs:  maint.Resources,
	}
}

package test

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testtimeutils "github.com/ruko1202/maintmode/test/utils/time"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// Scope pairings — which combinations of global and resource scope conflict, and
// what resources each reports — live in TestConflictScopeMatrix, which covers all
// of them in one table against both queries. What is left here is the other axis:
// the time window, which the matrix holds constant.
func TestListConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := conflicts.NewStore(db)

	t.Run("overlapping windows conflict", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		sharedResource := testdbutils.MakeResource(ctx, t, resourcesStore)

		maint := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(sharedResource.ID),
		)
		// Starts an hour into ours and runs past its end: a partial overlap, so
		// the reported window is an intersection rather than either period.
		neighbor := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
			entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(sharedResource.ID),
		)

		conflicted, err := store.ConflictedMaints(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			Scope:         maint.Scope,
			PlannedPeriod: maint.PlannedPeriod,
			ResourceIDs:   maint.Resources,
		})
		require.NoError(t, err)

		actual, found := lo.Find(conflicted, func(c *entity.Conflict) bool {
			return c.MaintenanceID == neighbor.ID
		})
		require.True(t, found, "overlapping maintenances must conflict")

		require.Equal(t, &entity.Conflict{
			MaintenanceID: neighbor.ID,
			Title:         neighbor.Title,
			Scope:         neighbor.Scope,
			OverlapStart:  testtimeutils.OverlapStart(maint.PlannedPeriod, neighbor.PlannedPeriod),
			OverlapEnd:    testtimeutils.OverlapEnd(maint.PlannedPeriod, neighbor.PlannedPeriod),
		}, actual, "the reported window is the intersection of the two periods")
	})

	t.Run("adjacent windows do not conflict", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		sharedResource := testdbutils.MakeResource(ctx, t, resourcesStore)

		maint := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(sharedResource.ID),
		)
		// Begins exactly where ours ends. The period is a half-open range, so
		// touching endpoints are not an overlap — and this neighbor shares a
		// resource AND is global-scope-adjacent in every other respect, so only
		// the time check can exclude it.
		neighbor := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
			entity.NewPeriod(end, end.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
		)

		conflicted, err := store.ConflictedMaints(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			Scope:         maint.Scope,
			PlannedPeriod: maint.PlannedPeriod,
			ResourceIDs:   maint.Resources,
		})
		require.NoError(t, err)

		require.False(t,
			lo.ContainsBy(conflicted, func(c *entity.Conflict) bool {
				return c.MaintenanceID == neighbor.ID
			}),
			"a maintenance starting when ours ends must not conflict, "+
				"even though it is global-scope and would otherwise match everything")
	})
}

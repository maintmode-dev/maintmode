package apimaint

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// TestConflictResources_IndependentOfViewer pins the core of the contract: a
// conflict reports the resources of the maintenance it names, not the subset the
// viewer happens to share with it.
//
// One neighbor on {R1, R2} is observed from two different vantage points — a
// global-scope viewer holding nothing, and a resource-scoped viewer holding only
// R1. Both must see {R1, R2}.
//
// The resource-scoped viewer is the case that changes behavior for existing
// users: it used to see {R1} alone, and now sees R2 as well — a resource it has
// nothing to do with. That is intended. The field answers "what does this
// neighbor touch", which is what an approver needs in order to judge blast
// radius, and an answer that shifts depending on who is asking cannot serve that.
//
// Two traps this test avoids by construction:
//
//   - It asserts each viewer against the literal fixture set, never one viewer
//     against the other. Comparing the two results to each other would pass while
//     both were wrongly empty — which is exactly what the old behavior did to the
//     global-scope viewer.
//   - It compares as a set. The underlying query has no ORDER BY, and
//     SortResources runs only inside ConflictFingerprint, so response order is
//     unspecified; asserting on it would invent a guarantee the code does not make.
func TestConflictResources_IndependentOfViewer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	calendarSrv := initCalendarViewImpl(t)

	start, end := testdbutils.IsolatedPeriodBounds(t)

	r1 := createResource(ctx, t)
	r2 := createResource(ctx, t)

	// The neighbor must be approved: only planned and in-progress maintenances
	// are counted as conflicts at all.
	neighbor := createMaintenanceInWindow(ctx, t, impl, scopedMaintParams{
		scope:        apimodels.MaintenanceScopeResources,
		resources:    []*apimodels.ResourceRef{r1.Ref, r2.Ref},
		plannedStart: start,
		duration:     end.Sub(start),
	})
	approveWithLiveConflicts(ctx, t, impl, calendarSrv, neighbor)

	for _, tc := range []struct {
		name   string
		params scopedMaintParams
	}{
		{
			name: "global-scope viewer holding no resources",
			params: scopedMaintParams{
				scope:        apimodels.MaintenanceScopeGlobal,
				plannedStart: start,
				duration:     end.Sub(start),
			},
		},
		{
			// Partial overlap: shares R1, has never heard of R2.
			name: "resource-scoped viewer sharing only one resource",
			params: scopedMaintParams{
				scope:        apimodels.MaintenanceScopeResources,
				resources:    []*apimodels.ResourceRef{r1.Ref},
				plannedStart: start,
				duration:     end.Sub(start),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Safe to parallelize, but not for the obvious reason. The two cases
			// do not merely read the shared neighbor — each creates its own
			// maintenance in the SAME window, and the global-scope one overlaps
			// every maintenance in that window regardless of resources. What keeps
			// them from conflicting with each other is that both viewers stay
			// drafts, and ConflictedMaints counts only planned and in-progress
			// maintenances.
			//
			// So: approving a viewer here would make these cases see each other.
			// If a case ever needs to approve, drop t.Parallel() or give each its
			// own IsolatedPeriodBounds window.
			t.Parallel()

			viewer := createMaintenanceInWindow(ctx, t, impl, tc.params)

			conflicts := getMaintViewConflicts(ctx, t, calendarSrv, viewer.ID)
			neighborConflict, found := lo.Find(conflicts, func(c *calendardto.Conflict) bool {
				return c.MaintenanceID == neighbor.ID
			})
			require.True(t, found, "the approved neighbor must appear as a conflict")

			require.Equal(t, entity.MaintenanceScopeResources, neighborConflict.Scope)

			// The assertion that carries this test: both viewers see the same two
			// ids, so the shared resource is not privileged over the unshared one
			// and the set does not depend on who is looking.
			actualIDs := lo.Map(neighborConflict.Resources,
				func(r *calendardto.MaintenanceResource, _ int) uuid.UUID { return r.ID },
			)
			require.ElementsMatch(t, []uuid.UUID{r1.Ref.ID, r2.Ref.ID}, actualIDs,
				"the neighbor's own resource set, whatever the viewer holds")

			// Names must resolve rather than degrade to the "unknown resource"
			// label — the ids now come from the neighbor rather than from the
			// viewer's own set, so resolvability is a new guarantee worth pinning.
			actualNames := lo.Map(neighborConflict.Resources,
				func(r *calendardto.MaintenanceResource, _ int) string { return r.Name },
			)
			require.ElementsMatch(t, []string{r1.Name, r2.Name}, actualNames)
		})
	}
}

// TestConflictResources_GlobalNeighborStaysEmpty closes the scope matrix: a
// global-scope neighbor legitimately owns no resources, so its conflict element
// still reports an empty list — and approve must still accept that element.
//
// This is the one cell where an empty `resources` stays legal after the change.
// Without it the fix would merely move the original bug: the approve validator
// requires a non-empty list only when the conflict's own scope is "resource", and
// a regression that widened that rule to every scope would make a maintenance
// with a global-scope neighbor unapprovable in exactly the way this work set out
// to fix.
//
// The approve half is the point. The store-level case covering the same scope
// pairing asserts only the map shape and cannot exercise the validator.
func TestConflictResources_GlobalNeighborStaysEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	calendarSrv := initCalendarViewImpl(t)

	start, end := testdbutils.IsolatedPeriodBounds(t)

	neighbor := createMaintenanceInWindow(ctx, t, impl, scopedMaintParams{
		scope:        apimodels.MaintenanceScopeGlobal,
		plannedStart: start,
		duration:     end.Sub(start),
	})
	approveWithLiveConflicts(ctx, t, impl, calendarSrv, neighbor)

	resource := createResource(ctx, t)
	subject := createMaintenanceInWindow(ctx, t, impl, scopedMaintParams{
		scope:        apimodels.MaintenanceScopeResources,
		resources:    []*apimodels.ResourceRef{resource.Ref},
		plannedStart: start,
		duration:     end.Sub(start),
	})

	conflicts := getMaintViewConflicts(ctx, t, calendarSrv, subject.ID)
	neighborConflict, found := lo.Find(conflicts, func(c *calendardto.Conflict) bool {
		return c.MaintenanceID == neighbor.ID
	})
	require.True(t, found, "the approved global-scope neighbor must appear as a conflict")

	require.Equal(t, entity.MaintenanceScopeGlobal, neighborConflict.Scope)
	require.Empty(t, neighborConflict.Resources,
		"a global-scope maintenance owns no resources, so there is nothing to report")
	// Empty but NOT nil: the difference is invisible in Go and decides whether the
	// wire carries [] or null, which the frontend consumes differently.
	require.NotNil(t, neighborConflict.Resources,
		"an empty resource list must serialize as [], never as null")

	// And the approve path accepts it.
	approveReq := &apimodels.ApproveDraftMaintRequest{
		ObservedMaintRevision: subject.CreatedAt.UnixMicro(),
		ConflictsSnapshot:     toAPIConflictSnapshot(conflicts),
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, approveReq),
	}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: subject.ID.String()},
	})
	xecho.UserToEchoCtx(c, approverUser(subject))

	err := impl.ApproveMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, rec.Code,
		"an empty resource list on a global-scope conflict is valid: %s", rec.Body.String())
}

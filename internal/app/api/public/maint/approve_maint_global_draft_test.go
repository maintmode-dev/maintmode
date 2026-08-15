package apimaint

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/calendar"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// TestApproveMaint_GlobalDraftWithResourceScopedNeighbor walks the whole
// round-trip that a user performs in the UI and that currently fails on the dev
// stand: view a global-scope draft, take the conflict list the server returned,
// send it straight back as conflicts_snapshot, approve.
//
// The neighbor is resource-scoped, so the read side reports it as
// `scope: "resource"` and now carries the neighbor's own resources. It used to
// carry the intersection with the viewed maintenance's resources instead, which
// for a global-scope draft — one that holds none — was always empty; the approve
// validator then rejected that very element for a blank `resources`, and the
// draft could not be approved by any means the UI had available.
//
// Unlike the API-level suite this test controls its own time window
// (IsolatedPeriodBounds), which matters twice over: a global-scope maintenance
// conflicts with EVERY overlapping maintenance regardless of resources, so
// without isolation this fixture would both pick up unrelated neighbors from
// parallel packages and inject itself into theirs. Isolation is also what makes
// asserting on approve possible at all here — checkConflicts recomputes the live
// set inside the transaction and refuses the approval if it moved, which is why
// the API-level conflict tests stop short of approving.
func TestApproveMaint_GlobalDraftWithResourceScopedNeighbor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	viewImpl := initCalendarViewImpl(t)

	start, end := testdbutils.IsolatedPeriodBounds(t)

	// The neighbor: resource-scoped and approved, since only planned and
	// in-progress maintenances are counted as conflicts at all.
	neighborResource := createResource(ctx, t)
	neighbor := createMaintenanceInWindow(ctx, t, impl, scopedMaintParams{
		scope:        apimodels.MaintenanceScopeResources,
		resources:    []*apimodels.ResourceRef{neighborResource.Ref},
		plannedStart: start,
		duration:     end.Sub(start),
	})
	approveWithLiveConflicts(ctx, t, impl, viewImpl, neighbor)

	// The subject: global-scope, so it owns no resources of its own.
	subject := createMaintenanceInWindow(ctx, t, impl, scopedMaintParams{
		scope:        apimodels.MaintenanceScopeGlobal,
		plannedStart: start,
		duration:     end.Sub(start),
	})

	// 1. Read the view exactly as the UI does.
	conflicts := getMaintViewConflicts(ctx, t, viewImpl, subject.ID)

	neighborConflict, found := lo.Find(conflicts, func(c *calendardto.Conflict) bool {
		return c.MaintenanceID == neighbor.ID
	})
	require.True(t, found, "the approved resource-scoped neighbor must appear as a conflict")

	// The shape the fix is about: a resource-scoped conflict now carries the
	// neighbor's OWN resources, even though the global-scope subject shares none
	// of them. Pinned explicitly so a regression in the read side breaks here
	// rather than further down in the approve assertion.
	require.Equal(t, entity.MaintenanceScopeResources, neighborConflict.Scope,
		"the neighbor is resource-scoped and the view must say so")
	require.Len(t, neighborConflict.Resources, 1,
		"the neighbor's own resource must be reported even though the global-scope "+
			"subject holds no resources of its own")
	require.Equal(t, neighborResource.Ref.ID, neighborConflict.Resources[0].ID)
	require.Equal(t, neighborResource.Name, neighborConflict.Resources[0].Name,
		"the resource name must resolve, not degrade to the unknown-resource label")

	// 2. Echo the conflict list back verbatim, which is all the UI can do.
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

	// 3. The server must accept the list it just produced.
	require.Equal(t, http.StatusNoContent, rec.Code,
		"approving a global-scope draft by echoing back the server's own conflict "+
			"list must not be rejected as invalid: %s", rec.Body.String())

	maint := getMaintByID(t, impl, subject.ID)
	require.Equal(t, string(entity.MaintenanceStatusPlanned), maint.Status)
}

// scopedMaintParams describes a maintenance fixture whose scope, resources and
// time window the caller controls. The shared createDraftMaintenance helper
// hardcodes all three, which the conflict cases here cannot use.
type scopedMaintParams struct {
	scope        apimodels.MaintenanceScope
	resources    []*apimodels.ResourceRef
	plannedStart time.Time
	duration     time.Duration
}

func createMaintenanceInWindow(
	ctx context.Context,
	t *testing.T,
	impl *Implementation,
	params scopedMaintParams,
) *apimodels.CreateDraftMaintResponse {
	t.Helper()

	notifyChan := makeNotifyChannel(ctx, t)

	req := &apimodels.CreateDraftMaintRequest{
		Title:        "Conflict fixture " + uuid.New().String()[:8],
		Description:  "Scoped maintenance fixture for conflict tests",
		PlannedStart: params.plannedStart,
		Scope:        params.scope,
		Impact:       apimodels.MaintenanceImpactNone,
		Resources:    params.resources,
		Steps: []*apimodels.MaintenanceStepInput{
			{
				Order:               1,
				Description:         "Step 1",
				RollbackDescription: "Rollback step 1",
				Duration:            params.duration.String(),
			},
		},
		NotifyTargets: &apimodels.NotifyTargets{
			ChannelIDs: []string{notifyChan.ID},
		},
		ApproverUserID: seedApprover(ctx, t),
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	xecho.UserToEchoCtx(c, makeUser(t))

	err := impl.CreateDraftMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	resp := testjsonudils.JSONToAny[apimodels.CreateDraftMaintResponse](t, rec.Body)
	return &resp
}

// getMaintViewConflicts returns the conflict list the UI read-view reports for a
// maintenance — the same call, through the same service, that fills the card the
// approver looks at before pressing Approve.
func getMaintViewConflicts(
	ctx context.Context,
	t *testing.T,
	calendarSrv *calendar.Service,
	maintID uuid.UUID,
) []*calendardto.Conflict {
	t.Helper()

	maint, err := calendarSrv.GetMaint(ctx, maintID)
	require.NoError(t, err)

	conflicts, err := calendarSrv.GetConflicts(ctx, &calendardto.ConflictQueryCmd{
		MaintID:       maintID,
		PlannedPeriod: maint.PlannedPeriod,
		Scope:         maint.Scope,
		ResourceIDs: lo.Map(maint.Resources, func(item *calendardto.MaintenanceResource, _ int) uuid.UUID {
			return item.ID
		}),
	})
	require.NoError(t, err)

	return conflicts
}

func initCalendarViewImpl(t *testing.T) *calendar.Service {
	t.Helper()

	services := testbootstraputils.InitServicesT(context.Background(), t, db, valkey, cfg)
	return services.Calendar
}

// approveWithLiveConflicts approves a maintenance the way the UI does: read the
// conflict list the server reports, then echo it back as the snapshot.
//
// The shared approveMaint helper sends an empty conflicts_snapshot, which only
// works for a maintenance that has no conflicts. Fixtures here deliberately
// overlap each other, and — because IsolatedPeriodBounds hands each test a
// deterministic window — they also overlap the leftovers of every previous run
// against the same database. So a fixture's conflict set is never reliably empty,
// and an empty snapshot loses to the approve gate with
// ErrConflictsChangedSincePreview.
//
// Caller precondition: there is a real gap between reading the conflicts and
// approving, and checkConflicts recomputes the set inside its transaction — so a
// maintenance appearing in the window in between would make this fail. Callers
// are safe only because IsolatedPeriodBounds hands each test a private window a
// century out. Do not reuse this against a window shared with another test.
func approveWithLiveConflicts(
	ctx context.Context,
	t *testing.T,
	impl *Implementation,
	calendarSrv *calendar.Service,
	maint *apimodels.CreateDraftMaintResponse,
) {
	t.Helper()

	conflicts := getMaintViewConflicts(ctx, t, calendarSrv, maint.ID)

	approveReq := &apimodels.ApproveDraftMaintRequest{
		ObservedMaintRevision: maint.CreatedAt.UnixMicro(),
		ConflictsSnapshot:     toAPIConflictSnapshot(conflicts),
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, approveReq),
	}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: maint.ID.String()},
	})
	xecho.UserToEchoCtx(c, approverUser(maint))

	err := impl.ApproveMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}

// toAPIConflictSnapshot maps a read-side conflict list into the approve request
// shape. The round trip is by id: the view returns resources as {id, name}
// objects while approve binds bare refs, so the echo is a mapping rather than a
// literal copy.
func toAPIConflictSnapshot(conflicts []*calendardto.Conflict) []*apimodels.Conflict {
	return lo.Map(conflicts, func(c *calendardto.Conflict, _ int) *apimodels.Conflict {
		return &apimodels.Conflict{
			MaintenanceID: c.MaintenanceID,
			OverlapStart:  c.OverlapStart,
			OverlapEnd:    c.OverlapEnd,
			Scope:         apimodels.MaintenanceScope(c.Scope),
			Resources: lo.Map(c.Resources, func(r *calendardto.MaintenanceResource, _ int) *apimodels.ResourceRef {
				return &apimodels.ResourceRef{ID: r.ID}
			}),
		}
	})
}

package uicalendar

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	uimodels "github.com/ruko1202/maintmode/internal/app/api/ui/calendar/models"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// This is the seam test the whole feature hangs on, and it is the one place it
// can be caught: the handler decides which question the conflict list answers
// by passing status and actual period into the query cmd. Omitting either is
// silent — a zero status is deliberately non-terminal, so the read falls back
// to the live path, every card keeps showing the old answer, and every
// store/service test stays green. Deleting those two lines from maint_view.go
// must fail HERE or nowhere.
func TestMaintViewReportsFactualConflicts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	maintStore := maintenances.NewStore(db)
	resourcesStore := resources.NewStore(db)

	start, end := testdbutils.IsolatedPeriodBounds(t)
	period := entity.NewPeriod(start, end)
	shared := testdbutils.MakeResource(ctx, t, resourcesStore)

	subject := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore, period,
		testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
		testdbutils.WithActualPeriod(period),
		testdbutils.WithScope(entity.MaintenanceScopeResources),
		testdbutils.WithResources(shared.ID),
	)

	// Ran alongside, then was canceled. The live query rejects it on status, so
	// it can only appear if the card took the factual branch.
	neighborPeriod := entity.NewPeriod(start.Add(time.Hour), end)
	neighbor := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore, neighborPeriod,
		testdbutils.WithStatus(entity.MaintenanceStatusCancelled),
		testdbutils.WithActualPeriod(neighborPeriod),
		testdbutils.WithScope(entity.MaintenanceScopeResources),
		testdbutils.WithResources(shared.ID),
	)

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))
	c.SetPathValues(echo.PathValues{{Name: "id", Value: subject.ID.String()}})

	require.NoError(t, impl.MaintView(c))
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testjsonudils.JSONToAny[uimodels.MaintenanceViewResponse](t, rec.Body)

	reported, found := lo.Find(resp.Conflicts, func(c *uimodels.ConflictView) bool {
		return c.MaintenanceID == neighbor.ID
	})
	require.True(t, found,
		"a completed maintenance must report the canceled neighbor that actually ran "+
			"alongside it; if this fails, the handler is not passing status/actual period")
	require.False(t, reported.KnownAtApproval,
		"the neighbor was never in this maintenance's approval snapshot")
}

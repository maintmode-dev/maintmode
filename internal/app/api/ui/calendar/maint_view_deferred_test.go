package uicalendar

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	maintmodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	uimodels "github.com/ruko1202/maintmode/internal/app/api/ui/calendar/models"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// TestMaintViewDeferredNotificationsOrder pins the fire_at ASC ordering of the
// read-view end to end, from the store through calendar.GetMaint's own mapping
// to the JSON response.
//
// The pure mapping test in models/bind_test.go cannot cover this: it feeds an
// already-sorted slice into ToAPIMaintenanceView, so it stays green even if
// GetMaint reorders the rows it loads. Reminders are submitted here in reverse
// order so a passing assertion proves the ordering rather than echoing the
// input.
func TestMaintViewDeferredNotificationsOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	// Truncated to a second: fire_at round-trips through Postgres and JSON, and
	// the assertions compare instants, not monotonic clock readings.
	base := xtime.UTCNow().Add(24 * time.Hour).Truncate(time.Second)
	earliest := base.Add(-30 * time.Minute)
	latest := base.Add(-5 * time.Minute)

	maint := makeMaintWithDeferred(ctx, t, []*maintmodels.DeferredNotification{
		{FireAt: latest},
		{FireAt: earliest},
	})

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))
	c.SetPathValues(echo.PathValues{{Name: "id", Value: maint.ID.String()}})

	require.NoError(t, impl.MaintView(c))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	resp := testjsonudils.JSONToAny[uimodels.MaintenanceViewResponse](t, rec.Body)
	require.NotNil(t, resp.Maintenance)

	reminders := resp.Maintenance.DeferredNotifications
	require.Len(t, reminders, 2)

	require.True(t, reminders[0].FireAt.Equal(earliest),
		"reminders must come back fire_at ASC: got %s before %s", reminders[0].FireAt, reminders[1].FireAt)
	require.True(t, reminders[1].FireAt.Equal(latest),
		"reminders must come back fire_at ASC: got %s after %s", reminders[1].FireAt, reminders[0].FireAt)

	for i, reminder := range reminders {
		require.NotEqual(t, uuid.Nil, reminder.ID, "reminder %d must carry an id", i)
		require.False(t, reminder.Scheduled, "reminder %d must not be scheduled while the maintenance is a draft", i)
	}
}

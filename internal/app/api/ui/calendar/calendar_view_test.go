package uicalendar

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	uimodels "github.com/ruko1202/maintmode/internal/app/api/ui/calendar/models"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestCalendarView(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)

		// Scoped to this maintenance's own catalog channel, the way the
		// "filters by notify channel" subtest below does. The date window
		// alone is not an isolator: every maintenance any test ever created
		// sits in it, and the calendar caps the page at 1000 rows ordered by
		// planned_period DESC, so on a well-used database the row seeded here
		// falls off the end and the lookup below finds nothing.
		require.Len(t, maint.NotifyTargets.ChannelIDs, 1)

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"from":        []string{maint.PlannedPeriod.Start.Add(-time.Hour).Format(time.DateOnly)},
				"to":          []string{maint.PlannedPeriod.End.Add(time.Hour).Format(time.DateOnly)},
				"channel_ids": []string{maint.NotifyTargets.ChannelIDs[0]},
			},
		}.ToContextRecorder(t)

		err := impl.CalendarView(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[uimodels.CalendarViewResponse](t, rec.Body)
		require.GreaterOrEqual(t, resp.Meta.Count, int64(1))

		event, ok := lo.Find(resp.Events, func(item *uimodels.CalendarEvent) bool {
			return item.ID == maint.ID
		})
		require.True(t, ok)
		require.Equal(t, maint.Title, event.Title)
		require.Equal(t, maint.PlannedPeriod.Start, event.Start)
		require.Equal(t, lo.FromPtr(maint.PlannedPeriod.End), event.End)
		require.Equal(t, maint.Scope, event.Scope)
		require.Equal(t, maint.Impact, event.Impact)
		require.Equal(t, maint.Status, string(event.Status))
	})

	t.Run("filters by notify channel", func(t *testing.T) {
		t.Parallel()

		// Each makeMaint subscribes its maintenance to its own fresh catalog
		// channel; the create response echoes the catalog channel uuid in
		// channel_ids, which is exactly what the calendar filter takes.
		maint := makeMaint(ctx, t)
		other := makeMaint(ctx, t)

		require.Len(t, maint.NotifyTargets.ChannelIDs, 1)
		channelID := maint.NotifyTargets.ChannelIDs[0]

		c, rec := echotest.ContextConfig{
			QueryValues: url.Values{
				"from":        []string{maint.PlannedPeriod.Start.Add(-time.Hour).Format(time.DateOnly)},
				"to":          []string{maint.PlannedPeriod.End.Add(24 * time.Hour).Format(time.DateOnly)},
				"channel_ids": []string{channelID},
			},
		}.ToContextRecorder(t)

		err := impl.CalendarView(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[uimodels.CalendarViewResponse](t, rec.Body)

		_, foundMaint := lo.Find(resp.Events, func(item *uimodels.CalendarEvent) bool {
			return item.ID == maint.ID
		})
		require.True(t, foundMaint, "subscribed maintenance must match the channel filter")

		_, foundOther := lo.Find(resp.Events, func(item *uimodels.CalendarEvent) bool {
			return item.ID == other.ID
		})
		require.False(t, foundOther, "maintenance subscribed to another channel must be filtered out")
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		now := xtime.UTCNow()

		for _, tc := range []struct {
			name          string
			queryValues   url.Values
			expStatusCode int
		}{
			{
				name: "missing from parameter",
				queryValues: url.Values{
					"to": []string{now.Add(24 * time.Hour).Format(time.DateOnly)},
				},
				expStatusCode: http.StatusBadRequest,
			}, {
				name: "missing to parameter",
				queryValues: url.Values{
					"from": []string{now.Add(-24 * time.Hour).Format(time.DateOnly)},
				},
				expStatusCode: http.StatusBadRequest,
			}, {
				name: "to before from",
				queryValues: url.Values{
					"to":   []string{now.Add(-24 * time.Hour).Format(time.DateOnly)},
					"from": []string{now.Add(24 * time.Hour).Format(time.DateOnly)},
				},
				expStatusCode: http.StatusBadRequest,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				c, rec := echotest.ContextConfig{
					QueryValues: tc.queryValues,
				}.ToContextRecorder(t)

				err := impl.CalendarView(c)
				require.NoError(t, err)
				require.Equal(t, tc.expStatusCode, rec.Code)
			})
		}
	})
}

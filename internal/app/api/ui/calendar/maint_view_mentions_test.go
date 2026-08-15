package uicalendar

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	maintapi "github.com/ruko1202/maintmode/internal/app/api/public/maint"
	maintmodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	uimodels "github.com/ruko1202/maintmode/internal/app/api/ui/calendar/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// TestMaintViewMentions pins the calendar card's mention hydration end to end,
// from the store through calendar.GetMaint to the JSON response. This is the
// gate-11 test: the public detail path is a separate one, so a card that never
// hydrated its mentions would stay green everywhere else.
func TestMaintViewMentions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	services := newServices(ctx, t)
	mentioned := testbootstraputils.SeedEligibleApprover(ctx, t, services)

	maint := makeMaintWithMentions(ctx, t, []uuid.UUID{mentioned.ID})

	resp, raw := maintViewResponse(ctx, t, impl, maint.ID)
	require.NotNil(t, resp.Maintenance)

	require.Len(t, resp.Maintenance.Mentions, 1)
	require.Equal(t, mentioned.ID, resp.Maintenance.Mentions[0].UserID)
	require.Equal(t, mentioned.Name, resp.Maintenance.Mentions[0].DisplayName)
	require.NotEqual(t, entity.UnknownUserName, resp.Maintenance.Mentions[0].DisplayName,
		"a real mentioned user must be named, not degraded")

	// Negative assertion on the raw body: the card must never carry the
	// messenger-tag flag or any tag value, and this view is readable by guests.
	require.NotContains(t, raw, "has_messenger_tag")
	require.NotContains(t, raw, "telegram_tag")
	require.NotContains(t, raw, "slack_tag")
}

// TestMaintViewMentionsEmpty pins the [] contract on the real response body.
func TestMaintViewMentionsEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	maint := makeMaintWithMentions(ctx, t, nil)

	resp, raw := maintViewResponse(ctx, t, impl, maint.ID)
	require.NotNil(t, resp.Maintenance)
	require.NotNil(t, resp.Maintenance.Mentions)
	require.Empty(t, resp.Maintenance.Mentions)
	require.Contains(t, raw, `"mentions":[]`, "an empty mention list must serialize as [], not null")
}

// TestCalendarViewEventsCarryNoMentions pins the deliberate omission: the event
// list hydrates no child collections at all, which is also what keeps it free of
// an N+1 read. Mentions belong to the card, not the list.
func TestCalendarViewEventsCarryNoMentions(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(&uimodels.CalendarEvent{ID: uuid.New(), Title: "t"})
	require.NoError(t, err)

	require.NotContains(t, string(raw), "mentions",
		"the calendar list event must not grow a mentions field")
}

// maintViewResponse fetches the card and returns both the decoded response and
// the raw body, so a test can assert on the serialized shape (an absent key is
// invisible after decoding into a typed struct).
func maintViewResponse(
	ctx context.Context,
	t *testing.T,
	impl *Implementation,
	maintID uuid.UUID,
) (resp *uimodels.MaintenanceViewResponse, rawBody string) {
	t.Helper()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))
	c.SetPathValues(echo.PathValues{{Name: "id", Value: maintID.String()}})

	require.NoError(t, impl.MaintView(c))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	raw := rec.Body.String()
	decoded := testjsonudils.JSONToAny[uimodels.MaintenanceViewResponse](t, rec.Body)

	return &decoded, raw
}

// makeMaintWithMentions seeds a draft maintenance tagging the given users. It
// mirrors makeMaintWithDeferred; the mentioned ids must be real users because
// the create path validates their eligibility against the real user backend.
func makeMaintWithMentions(
	ctx context.Context,
	t *testing.T,
	mentioned []uuid.UUID,
) *maintmodels.CreateDraftMaintResponse {
	t.Helper()

	services := newServices(ctx, t)
	maintImpl := maintapi.New(services.Maint, services.UserSummary)

	approver := testbootstraputils.SeedEligibleApprover(ctx, t, services)
	notifyChan := makeNotifyChannel(ctx, t)

	req := &maintmodels.CreateDraftMaintRequest{
		Title:        "Test maint for calendar mentions " + uuid.New().String()[:8],
		Description:  "Test description",
		PlannedStart: xtime.UTCNow().Add(futurePlannedStartOffset),
		Scope:        maintmodels.MaintenanceScopeGlobal,
		Impact:       maintmodels.MaintenanceImpactNone,
		Steps: []*maintmodels.MaintenanceStepInput{{
			Order:               1,
			Description:         "Step 1",
			RollbackDescription: "Rollback 1",
			Duration:            "30m",
		}},
		NotifyTargets: &maintmodels.NotifyTargets{
			ChannelIDs: []string{notifyChan.ID},
		},
		Mentions: lo.Map(mentioned, func(id uuid.UUID, _ int) *maintmodels.Mention {
			return &maintmodels.Mention{UserID: id}
		}),
		ApproverUserID: approver.ID,
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))
	xecho.UserToEchoCtx(c, &entity.User{
		ID:    uuid.New(),
		Email: "author-" + uuid.NewString() + "@example.com",
		Name:  "Author " + t.Name(),
		Roles: entity.DefaultRoles,
	})

	require.NoError(t, maintImpl.CreateDraftMaint(c))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	resp := testjsonudils.JSONToAny[maintmodels.CreateDraftMaintResponse](t, rec.Body)

	return &resp
}

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
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

// An invalid link is a 400 that names the field. The request is otherwise
// valid, so the status cannot come from some other rule.
func TestCreateDraftMaintRefusesInvalidTaskTrackerURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	req := createDraftRequest(ctx, t)
	req.TaskTrackerURL = "ftp://jira.example.com/OPS-42"

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	xecho.UserToEchoCtx(c, makeUser(t))

	require.NoError(t, impl.CreateDraftMaint(c))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "task_tracker_url")
}

// The update field is tri-state, and the dangerous mistake is mapping "absent"
// to "": every edit from a client that never sends the field would wipe the
// link. Each case starts from a draft that has one.
func TestUpdateDraftMaintTaskTrackerURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)
	const link = "https://jira.example.com/browse/OPS-42"

	draftWithLink := func(t *testing.T) uuid.UUID {
		t.Helper()

		req := createDraftRequest(ctx, t)
		req.TaskTrackerURL = link
		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)
		xecho.UserToEchoCtx(c, makeUser(t))
		require.NoError(t, impl.CreateDraftMaint(c))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		return testjsonudils.JSONToAny[apimodels.CreateDraftMaintResponse](t, rec.Body).ID
	}

	for name, tc := range map[string]struct {
		sent *string
		want string
	}{
		"absent keeps the link": {sent: nil, want: link},
		"empty clears it":       {sent: lo.ToPtr(""), want: ""},
		"new link replaces it":  {sent: lo.ToPtr("https://x.io/t/1"), want: "https://x.io/t/1"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			maintID := draftWithLink(t)
			updateDraftMaint(ctx, t, impl, maintID, func(req *apimodels.UpdateDraftMaintRequest) {
				req.TaskTrackerURL = tc.sent
			})

			require.Equal(t, tc.want, getMaintByID(t, impl, maintID).TaskTrackerURL)
		})
	}

	t.Run("invalid link is a 400 naming the field", func(t *testing.T) {
		t.Parallel()

		maintID := draftWithLink(t)
		req := validUpdateRequest(nil)
		req.NotifyTargets = &apimodels.NotifyTargets{ChannelIDs: []string{makeNotifyChannel(ctx, t).ID}}
		req.TaskTrackerURL = lo.ToPtr("https://u:p@x.io/t/1")

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{{Name: "id", Value: maintID.String()}})
		xecho.UserToEchoCtx(c, makeUser(t))

		require.NoError(t, impl.UpdateDraftMaint(c))
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "task_tracker_url")
		require.Equal(t, link, getMaintByID(t, impl, maintID).TaskTrackerURL)
	})
}

// A maintenance without a link still carries the key, as "": clients read the
// field unconditionally, so an omitempty tag dropping it would break them while
// a decoded struct would hide the difference. Hence the raw JSON.
func TestCreateDraftMaintWithoutTaskTrackerURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, createDraftRequest(ctx, t)),
	}.ToContextRecorder(t)
	xecho.UserToEchoCtx(c, makeUser(t))
	require.NoError(t, impl.CreateDraftMaint(c))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	created := testjsonudils.JSONToAny[map[string]any](t, rec.Body)
	require.Contains(t, created, "task_tracker_url")
	require.Equal(t, "", created["task_tracker_url"])

	rawID, ok := created["id"].(string)
	require.True(t, ok, "id must be a string, got %T", created["id"])
	c, rec = echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: rawID}})
	require.NoError(t, impl.GetMaint(c))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := testjsonudils.JSONToAny[map[string]any](t, rec.Body)
	require.Contains(t, got, "task_tracker_url")
	require.Equal(t, "", got["task_tracker_url"])
}

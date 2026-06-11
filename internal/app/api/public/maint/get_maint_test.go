package apimaint

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/entity"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestGetMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})

		err := impl.GetMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.Maintenance](t, rec.Body)
		require.Equal(t, draft.ID, resp.ID)
		require.Equal(t, draft.Title, resp.Title)
		require.Equal(t, "draft", resp.Status)

		// created_by is resolved from auth on read and always present, carrying
		// the author id from the create response. display_name is the resolved
		// name or the "Unknown user" label when auth cannot resolve it — never
		// failing the read.
		require.NotNil(t, resp.CreatedBy)
		require.Equal(t, draft.CreatedBy.ID, resp.CreatedBy.ID)
		require.NotEmpty(t, resp.CreatedBy.DisplayName)

		// notify_targets carries the catalog-resolved chips (id + name +
		// transport) for the read-only Notify channels section. The id is the
		// catalog channel uuid the draft was created with (round-trip with the
		// create request's channel_ids); the draft used one telegram channel
		// named after the test (see makeNotifyChannel).
		require.Len(t, resp.NotifyTargets, 1)
		require.Len(t, draft.NotifyTargets.ChannelIDs, 1)
		require.Equal(t, draft.NotifyTargets.ChannelIDs[0], resp.NotifyTargets[0].ID.String())
		require.NotEqual(t, uuid.Nil, resp.NotifyTargets[0].ID)
		require.Equal(t, t.Name(), resp.NotifyTargets[0].Name)
		require.Equal(t, string(entity.NotifyTransportTelegram), resp.NotifyTargets[0].Transport)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "not-a-uuid"},
		})

		err := impl.GetMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "00000000-0000-0000-0000-000000000000"},
		})

		err := impl.GetMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

package apimaint

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestApproveMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		approveReq := &apimodels.ApproveDraftMaintRequest{
			ObservedMaintRevision: draft.CreatedAt.UnixMicro(),
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, approveReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})

		err := impl.ApproveMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		maint := getMaintByID(t, impl, draft.ID)
		requireMaintStillMatchesDraft(t, draft, maint)
		require.Equal(t, string(entity.MaintenanceStatusPlanned), maint.Status)
		require.Nil(t, maint.ActualPeriod)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		t.Parallel()

		approveReq := &apimodels.ApproveDraftMaintRequest{
			ObservedMaintRevision: 1,
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, approveReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "invalid-uuid"},
		})

		err := impl.ApproveMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing observed revision", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		approveReq := &apimodels.ApproveDraftMaintRequest{}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, approveReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})

		err := impl.ApproveMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("cannot approve already planned", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		approveReq := &apimodels.ApproveDraftMaintRequest{
			ObservedMaintRevision: draft.CreatedAt.UnixMicro(),
		}

		// First approve succeeds
		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, approveReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})

		err := impl.ApproveMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		maint := getMaintByID(t, impl, draft.ID)
		require.Equal(t, string(entity.MaintenanceStatusPlanned), maint.Status)

		// Second approve fails (conflict)
		c2, rec2 := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, approveReq),
		}.ToContextRecorder(t)
		c2.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})

		err = impl.ApproveMaint(c2)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, rec2.Code)

		maint = getMaintByID(t, impl, draft.ID)
		require.Equal(t, string(entity.MaintenanceStatusPlanned), maint.Status)
	})
}

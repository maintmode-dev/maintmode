package apimaint

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestCancelMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		cancelReq := &apimodels.CancelMaintRequest{
			Reason:  apimodels.MaintenanceCancelReasonBusinessDecision,
			Comment: "No longer needed",
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, cancelReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.CancelMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		maint := getMaintByID(t, impl, draft.ID)
		requireMaintStillMatchesDraft(t, draft, maint)
		require.Equal(t, string(entity.MaintenanceStatusCancelled), maint.Status)
		require.Equal(t, cancelReq.Reason, maint.CancelReason)
		require.Equal(t, cancelReq.Comment, maint.CancelReasonComment)
		require.Nil(t, maint.ActualPeriod)
	})

	t.Run("already canceled is idempotent", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		cancelReq := &apimodels.CancelMaintRequest{
			Reason:  apimodels.MaintenanceCancelReasonBusinessDecision,
			Comment: "No longer needed",
		}

		// First cancel succeeds
		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, cancelReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.CancelMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		maint := getMaintByID(t, impl, draft.ID)
		require.Equal(t, string(entity.MaintenanceStatusCancelled), maint.Status)
		require.Equal(t, cancelReq.Reason, maint.CancelReason)
		require.Equal(t, cancelReq.Comment, maint.CancelReasonComment)

		// Second cancel stays successful because cancellation is idempotent.
		c2, rec2 := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, cancelReq),
		}.ToContextRecorder(t)
		c2.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})
		xecho.UserToEchoCtx(c2, makeUser(t))

		err = impl.CancelMaint(c2)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec2.Code)

		maint = getMaintByID(t, impl, draft.ID)
		require.Equal(t, string(entity.MaintenanceStatusCancelled), maint.Status)
		require.Equal(t, cancelReq.Reason, maint.CancelReason)
		require.Equal(t, cancelReq.Comment, maint.CancelReasonComment)
	})

	t.Run("validation error", func(t *testing.T) {
		t.Parallel()

		t.Run("invalid uuid", func(t *testing.T) {
			t.Parallel()

			cancelReq := &apimodels.CancelMaintRequest{
				Reason:  apimodels.MaintenanceCancelReasonBusinessDecision,
				Comment: "No longer needed",
			}

			c, rec := echotest.ContextConfig{
				JSONBody: testjsonudils.AnyToJSONBytes(t, cancelReq),
			}.ToContextRecorder(t)
			c.SetPathValues(echo.PathValues{
				{Name: "id", Value: "invalid-uuid"},
			})
			xecho.UserToEchoCtx(c, makeUser(t))

			err := impl.CancelMaint(c)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})

		t.Run("missing reason", func(t *testing.T) {
			t.Parallel()

			draft := createDraftMaintenance(ctx, t, impl)

			cancelReq := &apimodels.CancelMaintRequest{
				Comment: "No longer needed",
			}

			c, rec := echotest.ContextConfig{
				JSONBody: testjsonudils.AnyToJSONBytes(t, cancelReq),
			}.ToContextRecorder(t)
			c.SetPathValues(echo.PathValues{
				{Name: "id", Value: draft.ID.String()},
			})
			xecho.UserToEchoCtx(c, makeUser(t))

			err := impl.CancelMaint(c)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	})
}

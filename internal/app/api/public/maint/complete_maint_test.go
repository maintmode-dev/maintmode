package apimaint

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

func TestCompleteMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		approveMaint(t, impl, draft)
		startMaint(t, impl, draft)
		startStep(t, impl, draft.ID, draft.Steps[0].ID)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: draft.Steps[0].ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.CompleteStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		c, rec = echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err = impl.CompleteMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		maint := getMaintByID(t, impl, draft.ID)
		requireMaintStillMatchesDraft(t, draft, maint)
		require.Equal(t, "completed", maint.Status)
		require.NotNil(t, maint.ActualPeriod)
		require.False(t, maint.ActualPeriod.Start.IsZero())
		require.NotNil(t, maint.ActualPeriod.End)
		require.Equal(t, apimodels.MaintenanceStepStatusCompleted, maint.Steps[0].Status)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "invalid-uuid"},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.CompleteMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("cannot complete draft", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.CompleteMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("cannot complete with unfinished steps", func(t *testing.T) {
		t.Parallel()

		// Maintenance is in_progress but its step is left non-terminal (pending).
		// This is a domain precondition failure, not an internal error: it must
		// surface as 409 with a stable code, never a 500 "internal error".
		draft := createDraftMaintenance(ctx, t, impl)

		approveMaint(t, impl, draft)
		startMaint(t, impl, draft)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.CompleteMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, rec.Code)
		require.Contains(t, rec.Body.String(), string(httperrors.ErrStepsNotTerminal))
	})
}

package apimaint

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

func TestStartMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		approveMaint(t, impl, draft)

		// Start (planned -> in_progress)
		c2, rec2 := echotest.ContextConfig{}.ToContextRecorder(t)
		c2.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})
		xecho.UserToEchoCtx(c2, makeUser(t))

		err := impl.StartMaint(c2)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec2.Code, rec2.Body.String())

		maint := getMaintByID(t, impl, draft.ID)
		requireMaintStillMatchesDraft(t, draft, maint)
		require.Equal(t, "in_progress", maint.Status)
		require.NotNil(t, maint.ActualPeriod)
		require.False(t, maint.ActualPeriod.Start.IsZero())
		require.Nil(t, maint.ActualPeriod.End)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "invalid-uuid"},
		})

		err := impl.StartMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("cannot start draft", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.StartMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, rec.Code)

		maint := getMaintByID(t, impl, draft.ID)
		requireMaintStillMatchesDraft(t, draft, maint)
		require.Equal(t, "draft", maint.Status)
		require.Nil(t, maint.ActualPeriod)
	})
}

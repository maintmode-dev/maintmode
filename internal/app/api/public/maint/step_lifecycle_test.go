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
)

func TestFullStepLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("to start", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)
		approveMaint(t, impl, draft)
		startMaint(t, impl, draft)

		stepID := draft.Steps[0].ID

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: stepID.String()},
		})

		err := impl.StartStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		maint := getMaintByID(t, impl, draft.ID)
		requireMaintStillMatchesDraft(t, draft, maint)
		require.Equal(t, string(entity.MaintenanceStatusInProgress), maint.Status)
		require.Equal(t, apimodels.MaintenanceStepStatusStarted, maint.Steps[0].Status)
	})

	t.Run("to complete", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)
		approveMaint(t, impl, draft)
		startMaint(t, impl, draft)

		stepID := draft.Steps[0].ID
		startStep(t, impl, draft.ID, stepID)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: stepID.String()},
		})

		err := impl.CompleteStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		maint := getMaintByID(t, impl, draft.ID)
		requireMaintStillMatchesDraft(t, draft, maint)
		require.Equal(t, string(entity.MaintenanceStatusInProgress), maint.Status)
		require.Equal(t, apimodels.MaintenanceStepStatusCompleted, maint.Steps[0].Status)
	})

	t.Run("to cancel", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)
		approveMaint(t, impl, draft)
		startMaint(t, impl, draft)

		stepID := draft.Steps[0].ID
		startStep(t, impl, draft.ID, stepID)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: stepID.String()},
		})

		err := impl.CancelStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		maint := getMaintByID(t, impl, draft.ID)
		requireMaintStillMatchesDraft(t, draft, maint)
		require.Equal(t, string(entity.MaintenanceStatusInProgress), maint.Status)
		require.Equal(t, apimodels.MaintenanceStepStatusCanceled, maint.Steps[0].Status)
	})
}
func TestStartStep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("invalid maint uuid", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "invalid-uuid"},
			{Name: "step_id", Value: "00000000-0000-0000-0000-000000000000"},
		})

		err := impl.StartStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid step uuid", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: "invalid-uuid"},
		})

		err := impl.StartStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCompleteStep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("invalid maint uuid", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "invalid-uuid"},
			{Name: "step_id", Value: "00000000-0000-0000-0000-000000000000"},
		})

		err := impl.CompleteStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid step uuid", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: "invalid-uuid"},
		})

		err := impl.CompleteStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCancelStep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("invalid maint uuid", func(t *testing.T) {
		t.Parallel()

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "invalid-uuid"},
			{Name: "step_id", Value: "00000000-0000-0000-0000-000000000000"},
		})

		err := impl.CancelStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid step uuid", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: "invalid-uuid"},
		})

		err := impl.CancelStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

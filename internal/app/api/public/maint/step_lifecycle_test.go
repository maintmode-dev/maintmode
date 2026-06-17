package apimaint

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"

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
		xecho.UserToEchoCtx(c, makeUser(t))

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
		xecho.UserToEchoCtx(c, makeUser(t))

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
		xecho.UserToEchoCtx(c, makeUser(t))

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
		xecho.UserToEchoCtx(c, makeUser(t))

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
		xecho.UserToEchoCtx(c, makeUser(t))

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
		xecho.UserToEchoCtx(c, makeUser(t))

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
		xecho.UserToEchoCtx(c, makeUser(t))

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
		xecho.UserToEchoCtx(c, makeUser(t))

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
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.CancelStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// Domain rejections on the step lifecycle must surface as a stable 409
// "forbidden status", never a 400 (looks like a client/validation bug) or a
// 500 (indistinguishable from a real crash and noisy for alerting).
func TestStartStep_DomainRejections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("step action when maintenance not in_progress -> 409", func(t *testing.T) {
		t.Parallel()

		// Approved but NOT started: maintenance is planned, not in_progress.
		// Starting a step is a domain precondition failure, not a crash.
		draft := createDraftMaintenance(ctx, t, impl)
		approveMaint(t, impl, draft)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: draft.Steps[0].ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.StartStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), string(httperrors.ErrForbiddenStatusTransition))
	})

	t.Run("out-of-order start -> 409", func(t *testing.T) {
		t.Parallel()

		// Maintenance in_progress; start step 2 while step 1 is still pending.
		// Step-order violation is a domain rejection, never a 500.
		draft := createTwoStepDraftMaintenance(ctx, t, impl)
		approveMaint(t, impl, draft)
		startMaint(t, impl, draft)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: draft.Steps[1].ID.String()}, // step order 2
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.StartStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), string(httperrors.ErrForbiddenStatusTransition))
	})
}

func TestCompleteStep_DomainRejections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("complete a pending step without starting -> 409", func(t *testing.T) {
		t.Parallel()

		// Maintenance in_progress, step still pending. Completing skips the
		// required started state -> forbidden transition, must be 409 not 400/500.
		draft := createDraftMaintenance(ctx, t, impl)
		approveMaint(t, impl, draft)
		startMaint(t, impl, draft)

		c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: draft.Steps[0].ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.CompleteStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), string(httperrors.ErrForbiddenStatusTransition))
	})

	t.Run("action on a terminal step -> 409", func(t *testing.T) {
		t.Parallel()

		// Drive the step to a terminal (canceled) state, then try to complete it.
		// No transition is legal out of a terminal step -> 409.
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
		require.NoError(t, impl.CancelStep(c))
		require.Equal(t, http.StatusNoContent, rec.Code)

		c, rec = echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
			{Name: "step_id", Value: draft.Steps[0].ID.String()},
		})
		xecho.UserToEchoCtx(c, makeUser(t))

		err := impl.CompleteStep(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), string(httperrors.ErrForbiddenStatusTransition))
	})
}

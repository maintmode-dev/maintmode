package apimaint

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestUpdateDraftMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		updateReq := &apimodels.UpdateDraftMaintRequest{
			Title:        "Updated title",
			Description:  "Updated description",
			PlannedStart: time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second),
			Scope:        apimodels.MaintenanceScopeGlobal,
			Impact:       apimodels.MaintenanceImpactFull,
			Steps: []*apimodels.MaintenanceStepInput{
				{
					Order:               1,
					Description:         "Updated step",
					RollbackDescription: "Updated rollback",
					Duration:            "45m",
				},
			},
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, updateReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})

		err := impl.UpdateDraftMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, rec.Code)

		c, rec = echotest.ContextConfig{}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})

		err = impl.GetMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		maint := testjsonudils.JSONToAny[apimodels.Maintenance](t, rec.Body)
		require.Equal(t, draft.ID, maint.ID)
		require.Equal(t, draft.CreatedAt.UnixMicro(), maint.CreatedAt.UnixMicro())
		require.Equal(t, draft.Status, maint.Status)
		require.Nil(t, maint.ActualPeriod)
		require.Empty(t, maint.CancelReason)
		require.Empty(t, maint.CancelReasonComment)

		require.Equal(t, updateReq.Title, maint.Title)
		require.Equal(t, updateReq.Description, maint.Description)
		require.Equal(t, updateReq.PlannedStart, maint.PlannedPeriod.Start)
		require.NotNil(t, maint.PlannedPeriod.End)
		require.Equal(t, updateReq.PlannedStart.Add(45*time.Minute), *maint.PlannedPeriod.End)
		require.Equal(t, updateReq.Scope, maint.Scope)
		require.Equal(t, updateReq.Impact, maint.Impact)
		require.Empty(t, maint.Resources)
		require.Len(t, maint.Steps, 1)
		require.Equal(t, updateReq.Steps[0].Order, maint.Steps[0].Order)
		require.Equal(t, updateReq.Steps[0].Description, maint.Steps[0].Description)
		require.Equal(t, updateReq.Steps[0].RollbackDescription, maint.Steps[0].RollbackDescription)
		require.Equal(t, int64(45), maint.Steps[0].DurationMinutes)
		require.Equal(t, apimodels.MaintenanceStepStatusPlanned, maint.Steps[0].Status)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		t.Parallel()

		updateReq := &apimodels.UpdateDraftMaintRequest{
			Title:        "Updated title",
			Description:  "Updated description",
			PlannedStart: time.Now().Add(48 * time.Hour),
			Scope:        apimodels.MaintenanceScopeGlobal,
			Impact:       apimodels.MaintenanceImpactFull,
			Steps: []*apimodels.MaintenanceStepInput{
				{
					Order:               1,
					Description:         "Step",
					RollbackDescription: "Rollback",
					Duration:            "10m",
				},
			},
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, updateReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: "invalid-uuid"},
		})

		err := impl.UpdateDraftMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing title", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		updateReq := &apimodels.UpdateDraftMaintRequest{
			Description:  "Updated description",
			PlannedStart: time.Now().Add(48 * time.Hour),
			Scope:        apimodels.MaintenanceScopeGlobal,
			Impact:       apimodels.MaintenanceImpactFull,
			Steps: []*apimodels.MaintenanceStepInput{
				{
					Order:               1,
					Description:         "Step",
					RollbackDescription: "Rollback",
					Duration:            "10m",
				},
			},
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, updateReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})

		err := impl.UpdateDraftMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing steps", func(t *testing.T) {
		t.Parallel()

		draft := createDraftMaintenance(ctx, t, impl)

		updateReq := &apimodels.UpdateDraftMaintRequest{
			Title:        "Updated title",
			Description:  "Updated description",
			PlannedStart: time.Now().Add(48 * time.Hour),
			Scope:        apimodels.MaintenanceScopeGlobal,
			Impact:       apimodels.MaintenanceImpactFull,
			Steps:        []*apimodels.MaintenanceStepInput{},
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, updateReq),
		}.ToContextRecorder(t)
		c.SetPathValues(echo.PathValues{
			{Name: "id", Value: draft.ID.String()},
		})

		err := impl.UpdateDraftMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

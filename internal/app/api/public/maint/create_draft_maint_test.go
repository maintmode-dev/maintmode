package apimaint

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

func TestCreateDraftMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	t.Run("ok global scope", func(t *testing.T) {
		t.Parallel()

		req := &apimodels.CreateDraftMaintRequest{
			Title:        "DB migration",
			Description:  "PostgreSQL major upgrade",
			PlannedStart: time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second),
			Scope:        apimodels.MaintenanceScopeGlobal,
			Impact:       apimodels.MaintenanceImpactNone,
			Steps: []*apimodels.MaintenanceStepInput{
				{
					Order:               1,
					Description:         "Run migration",
					RollbackDescription: "Rollback migration",
					Duration:            "30m",
				},
			},
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)

		err := impl.CreateDraftMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.CreateDraftMaintResponse](t, rec.Body)
		require.NotEqual(t, uuid.Nil, resp.ID)
		require.Equal(t, req.Title, resp.Title)
		require.Equal(t, req.Description, resp.Description)
		require.Equal(t, string(req.Scope), resp.Scope)
		require.Equal(t, string(req.Impact), resp.Impact)
		require.Equal(t, "draft", resp.Status)
		require.Len(t, resp.Steps, 1)
		require.NotEqual(t, uuid.Nil, resp.Steps[0].ID)

		maint := getMaintByID(t, impl, resp.ID)
		require.Equal(t, resp.ID, maint.ID)
		require.Equal(t, req.Title, maint.Title)
		require.Equal(t, req.Description, maint.Description)
		require.Equal(t, req.PlannedStart, maint.PlannedPeriod.Start)
		require.NotNil(t, maint.PlannedPeriod.End)
		require.Equal(t, req.PlannedStart.Add(30*time.Minute), *maint.PlannedPeriod.End)
		require.Equal(t, req.Scope, maint.Scope)
		require.Equal(t, req.Impact, maint.Impact)
		require.Equal(t, "draft", maint.Status)
		require.Empty(t, maint.Resources)
		require.Nil(t, maint.ActualPeriod)
		require.Empty(t, maint.CancelReason)
		require.Empty(t, maint.CancelReasonComment)
		require.Len(t, maint.Steps, 1)
		require.Equal(t, resp.Steps[0].ID, maint.Steps[0].ID)
		require.Equal(t, req.Steps[0].Order, maint.Steps[0].Order)
		require.Equal(t, req.Steps[0].Description, maint.Steps[0].Description)
		require.Equal(t, req.Steps[0].RollbackDescription, maint.Steps[0].RollbackDescription)
		require.Equal(t, int64(30), maint.Steps[0].DurationMinutes)
		require.Equal(t, apimodels.MaintenanceStepStatusPlanned, maint.Steps[0].Status)
	})

	t.Run("ok resource scope", func(t *testing.T) {
		t.Parallel()

		resource := createResource(ctx, t)
		req := &apimodels.CreateDraftMaintRequest{
			Title:        "Resource maintenance",
			Description:  "Service deploy",
			PlannedStart: time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second),
			Scope:        apimodels.MaintenanceScopeResources,
			Impact:       apimodels.MaintenanceImpactPartial,
			Resources: []*apimodels.ResourceRef{
				{ID: resource.ID},
			},
			Steps: []*apimodels.MaintenanceStepInput{
				{
					Order:               1,
					Description:         "Deploy",
					RollbackDescription: "Rollback deploy",
					Duration:            "15m",
				},
			},
		}

		c, rec := echotest.ContextConfig{
			JSONBody: testjsonudils.AnyToJSONBytes(t, req),
		}.ToContextRecorder(t)

		err := impl.CreateDraftMaint(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apimodels.CreateDraftMaintResponse](t, rec.Body)
		require.NotEqual(t, uuid.Nil, resp.ID)
		require.Equal(t, string(req.Scope), resp.Scope)
		require.Len(t, resp.Resources, 1)
		require.Equal(t, resource.ID, resp.Resources[0].ID)

		maint := getMaintByID(t, impl, resp.ID)
		require.Equal(t, resp.ID, maint.ID)
		require.Equal(t, req.Title, maint.Title)
		require.Equal(t, req.Description, maint.Description)
		require.Equal(t, req.Scope, maint.Scope)
		require.Equal(t, req.Impact, maint.Impact)
		require.Equal(t, "draft", maint.Status)
		require.Len(t, maint.Resources, 1)
		require.Equal(t, resource.ID, maint.Resources[0].ID)
		require.Len(t, maint.Steps, 1)
		require.Equal(t, req.Steps[0].Description, maint.Steps[0].Description)
		require.Equal(t, int64(15), maint.Steps[0].DurationMinutes)
	})

	t.Run("validation error", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name       string
			reqMutator func(req *apimodels.CreateDraftMaintRequest)
		}{
			{
				name: "missing title",
				reqMutator: func(req *apimodels.CreateDraftMaintRequest) {
					req.Title = ""
				},
			}, {
				name: "missing steps",
				reqMutator: func(req *apimodels.CreateDraftMaintRequest) {
					req.Steps = []*apimodels.MaintenanceStepInput{}
				},
			}, {
				name: "missing description in step",
				reqMutator: func(req *apimodels.CreateDraftMaintRequest) {
					req.Steps[0].Description = ""
				},
			}, {
				name: "resource scope without resources",
				reqMutator: func(req *apimodels.CreateDraftMaintRequest) {
					req.Resources = []*apimodels.ResourceRef{}
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				resource := createResource(ctx, t)
				req := &apimodels.CreateDraftMaintRequest{
					Title:        "Resource maintenance",
					Description:  "Service deploy",
					PlannedStart: time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second),
					Scope:        apimodels.MaintenanceScopeResources,
					Impact:       apimodels.MaintenanceImpactPartial,
					Resources: []*apimodels.ResourceRef{
						{ID: resource.ID},
					},
					Steps: []*apimodels.MaintenanceStepInput{
						{
							Order:               1,
							Description:         "Deploy",
							RollbackDescription: "Rollback deploy",
							Duration:            "15m",
						},
					},
				}
				tc.reqMutator(req)

				c, rec := echotest.ContextConfig{
					JSONBody: testjsonudils.AnyToJSONBytes(t, req),
				}.ToContextRecorder(t)

				err := impl.CreateDraftMaint(c)
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, rec.Code)
			})
		}
	})
}

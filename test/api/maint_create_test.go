//go:build api

package api

import (
	"context"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestMaintenancesAPI_CreateDraft(t *testing.T) {
	ctx := context.Background()
	apiClient := setupMaintmodeTestClient()

	resource := creatResource(ctx, t, apiClient)
	resourceID := strfmt.UUID(resource.ID)

	now := xtime.UTCNow()
	plannedStart := strfmt.DateTime(now.Add(24 * time.Hour).Truncate(time.Second))

	req := &models.ApimodelsCreateDraftMaintRequest{
		Title:        "Test Maintenance",
		Description:  "This is a test maintenance created via API tests",
		Impact:       models.ApimodelsMaintenanceImpactNone,
		Scope:        models.ApimodelsMaintenanceScopeResource,
		PlannedStart: plannedStart,
		Resources: []*models.ApimodelsResourceRef{
			{ID: resourceID},
		},
		Steps: testMaintenanceSteps(),
	}

	params := maintenances.NewPostAPIV1MaintenancesCreateParams().
		WithContext(ctx).
		WithRequest(req)

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesCreate(params, nil)
	require.NoError(t, err, "Failed to create maintenance draft")
	require.NotNil(t, resp, "Response should not be nil")

	payload := resp.Payload
	require.NotEmpty(t, payload.ID, "Maintenance ID should not be empty")
	require.Equal(t, "Test Maintenance", payload.Title)
	require.Equal(t, "This is a test maintenance created via API tests", payload.Description)
	require.Equal(t, string(models.UimodelsMaintenanceStatusDraft), payload.Status)
	require.False(t, payload.CreatedAt.IsZero())
	require.NotNil(t, payload.PlannedPeriod)
	require.Equal(t, plannedStart, payload.PlannedPeriod.Start)
	require.Equal(t, strfmt.DateTime(time.Time(plannedStart).Add(testMaintenanceDuration)), payload.PlannedPeriod.End)
}

//go:build api

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestMaintenancesAPI_CreateDraft(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)
	apiClient := setupMaintmodeTestClient()

	resource := creatResource(ctx, t, apiClient)
	resourceID := uuid.MustParse(lo.FromPtr(resource.Id))

	plannedStart := xtime.UTCNow().Add(24 * time.Hour).Truncate(time.Second)

	req := maintmodeclient.PostApiV1MaintenancesCreateJSONRequestBody{
		Title:        lo.ToPtr("Test Maintenance"),
		Description:  lo.ToPtr("This is a test maintenance created via API tests"),
		Impact:       lo.ToPtr(maintmodeclient.MaintenanceImpactNone),
		Scope:        lo.ToPtr(maintmodeclient.MaintenanceScopeResources),
		PlannedStart: lo.ToPtr(plannedStart),
		Resources: lo.ToPtr([]maintmodeclient.ApimodelsResourceRef{
			{Id: lo.ToPtr(resourceID)},
		}),
		Steps:         lo.ToPtr(testMaintenanceSteps()),
		NotifyTargets: testNotifyTargets(ctx, t, apiClient),
	}

	resp, err := apiClient.PostApiV1MaintenancesCreateWithResponse(ctx, req)
	require.NoError(t, err, "Failed to create maintenance draft")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	require.NotEmpty(t, lo.FromPtr(payload.Id), "Maintenance ID should not be empty")
	require.Equal(t, "Test Maintenance", lo.FromPtr(payload.Title))
	require.Equal(t, "This is a test maintenance created via API tests", lo.FromPtr(payload.Description))
	require.Equal(t, string(maintmodeclient.MaintenanceStatusDraft), lo.FromPtr(payload.Status))
	require.False(t, lo.FromPtr(payload.CreatedAt).IsZero())
	require.NotNil(t, payload.PlannedPeriod)
	require.Equal(t, plannedStart, lo.FromPtr(payload.PlannedPeriod.Start))
	require.Equal(t, plannedStart.Add(testMaintenanceDuration), lo.FromPtr(payload.PlannedPeriod.End))
}

//go:build api

package api

import (
	"context"
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
)

func TestMaintenancesAPI_GetByID(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	params := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	resp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(params)
	require.NoError(t, err, "Failed to get maintenance by ID")
	require.NotNil(t, resp, "Response should not be nil")

	payload := resp.Payload
	require.Equal(t, maintenanceID, payload.ID.String())
	require.Equal(t, "Test Maintenance", payload.Title)
}

func TestMaintenancesAPI_GetNonExistent(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	nonExistentID := strfmt.UUID(xuuid.New().String())
	params := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(nonExistentID)

	_, err := apiClient.Maintenances.GetAPIV1MaintenancesID(params)
	require.Error(t, err, "Should return error for non-existent maintenance")

	var apiErr *maintenances.GetAPIV1MaintenancesIDNotFound
	require.ErrorAs(t, err, &apiErr, "Expected GetAPIV1MaintenancesIDNotFound error")
	require.NotNil(t, apiErr.Payload, "Error payload should not be nil")
}

func TestMaintenancesAPI_InvalidUUID(t *testing.T) {
	ctx := context.Background()

	apiClient := setupMaintmodeTestClient()

	params := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID("invalid-uuid")

	_, err := apiClient.Maintenances.GetAPIV1MaintenancesID(params)
	require.Error(t, err, "Should return error for invalid UUID")

	var apiErr *maintenances.GetAPIV1MaintenancesIDBadRequest
	require.ErrorAs(t, err, &apiErr, "Expected GetAPIV1MaintenancesIDBadRequest error")
	require.NotNil(t, apiErr.Payload, "Error payload should not be nil")
}

//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xhttp"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestMaintenancesAPI_GetByID(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	maintenanceID := createTestMaintenance(ctx, t, apiClient)

	resp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance by ID")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	payload := resp.JSON200
	require.Equal(t, maintenanceID, lo.FromPtr(payload.Id).String())
	require.Equal(t, "Test Maintenance", lo.FromPtr(payload.Title))
}

func TestMaintenancesAPI_GetNonExistent(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	apiClient := setupMaintmodeTestClient()

	nonExistentID := xuuid.New()

	resp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, nonExistentID)
	require.NoError(t, err, "Should not return transport error for non-existent maintenance")
	require.Equal(t, http.StatusNotFound, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON404, "Error payload should not be nil")
}

func TestMaintenancesAPI_InvalidUUID(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	// The typed client serializes path UUIDs, so a malformed UUID can only be
	// exercised with a hand-rolled request against the maintenance path.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL("maintmode")+"/api/v1/maintenances/invalid-uuid", http.NoBody)
	require.NoError(t, err)
	xhttp.SetBearerToken(req, mustTestAccessToken())

	resp, err := xhttp.NewClient().Do(req)
	require.NoError(t, err, "Should not return transport error for invalid UUID")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "Expected bad request for invalid UUID")
}

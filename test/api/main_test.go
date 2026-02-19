//go:build api

package api_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	"github.com/ruko1202/maintmode/test/api/client/client"
	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

const (
	// Environment variables for test configuration
	envAPIHost     = "TEST_API_HOST"
	envAPIPort     = "TEST_API_PORT"
	envHealthCheck = "TEST_HEALTH_CHECK"

	// Default values
	defaultAPIHost = "localhost"
	defaultAPIPort = "8000"

	// Health check configuration
	healthCheckURL      = "http://localhost:8001/readiness"
	healthCheckTimeout  = 10 * time.Second
	healthCheckInterval = 100 * time.Millisecond

	// Test maintenance timing constants
	testMaintenanceStartOffset = 24 * time.Hour
	testMaintenanceDuration    = 2 * time.Hour
	testMaintenanceLongOffset  = 48 * time.Hour
)

func TestMain(m *testing.M) {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Panic(err)
	}
	xlog.ReplaceGlobal(l)
	ctx := xlog.ContextWithLogger(context.Background(), l)

	viper.AutomaticEnv()

	viper.SetDefault(envAPIHost, defaultAPIHost)
	viper.SetDefault(envAPIPort, defaultAPIPort)
	viper.SetDefault(envHealthCheck, false)

	if err := waitForAPIHealth(ctx); err != nil {
		xlog.Panic(ctx, "Failed to wait for API health check", zap.Error(err))
	}

	code := m.Run()
	os.Exit(code)
}

func waitForAPIHealth(ctx context.Context) error {
	xlog.Info(ctx, "Waiting for API to become healthy", zap.String("url", healthCheckURL))

	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("API did not become healthy within %v", healthCheckTimeout)
		case <-ticker.C:
			xlog.Info(ctx, "Ping service", zap.String("url", healthCheckURL))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthCheckURL, http.NoBody)
			if err != nil {
				xlog.Info(ctx, "Failed to create health check request", zap.Error(err))
				continue
			}

			resp, err := httpClient.Do(req)
			if err != nil {
				xlog.Info(ctx, "Health check failed", zap.Error(err))
				continue
			}

			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				xlog.Info(ctx, "API is healthy", zap.Int("status", resp.StatusCode))
				return nil
			}

			xlog.Info(ctx, "API not yet healthy", zap.Int("status", resp.StatusCode))
		}
	}
}

func setupTestClient() *client.Maintmode {
	transport := httptransport.New(
		fmt.Sprintf("%s:%s", viper.GetString(envAPIHost), viper.GetString(envAPIPort)),
		"/",
		[]string{"http"},
	)
	return client.New(transport, strfmt.Default)
}

func createTestMaintenance(ctx context.Context, t *testing.T, apiClient *client.Maintmode) string {
	t.Helper()

	resourceID := strfmt.UUID(xuuid.New().String())

	now := xtime.UTCNow()
	plannedStart := strfmt.DateTime(now.Add(testMaintenanceStartOffset))
	plannedEnd := strfmt.DateTime(now.Add(testMaintenanceStartOffset + testMaintenanceDuration))

	req := &models.ApimodelsCreateDraftMaintRequest{
		Title:       "Test Maintenance",
		Description: "Test maintenance for API testing",
		Impact:      models.ApimodelsMaintenanceImpactNone,
		Scope:       models.ApimodelsMaintenanceScopeResource,
		PlannedPeriod: &models.ApimodelsPeriod{
			Start: plannedStart,
			End:   plannedEnd,
		},
		Resources: []*models.ApimodelsResource{
			{
				ID:   resourceID,
				Type: models.ApimodelsResourceTypeDatabase,
			},
		},
	}

	params := maintenances.NewPostAPIV1MaintenancesCreateParams().
		WithContext(ctx).
		WithRequest(req)

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesCreate(params)
	require.NoError(t, err, "Failed to create test maintenance")
	require.NotNil(t, resp, "Response should not be nil")

	return resp.Payload.ID.String()
}

func createMaintenanceWithResource(ctx context.Context, t *testing.T, apiClient *client.Maintmode, resourceID string) string {
	t.Helper()

	resourceUUID := strfmt.UUID(resourceID)

	now := xtime.UTCNow()
	plannedStart := strfmt.DateTime(now.Add(testMaintenanceStartOffset))
	plannedEnd := strfmt.DateTime(now.Add(testMaintenanceStartOffset + testMaintenanceDuration))

	req := &models.ApimodelsCreateDraftMaintRequest{
		Title:       "Test Maintenance with Resource",
		Description: "Maintenance for testing resource-based operations",
		Impact:      models.ApimodelsMaintenanceImpactNone,
		Scope:       models.ApimodelsMaintenanceScopeResource,
		PlannedPeriod: &models.ApimodelsPeriod{
			Start: plannedStart,
			End:   plannedEnd,
		},
		Resources: []*models.ApimodelsResource{
			{
				ID:   resourceUUID,
				Type: models.ApimodelsResourceTypeService,
			},
		},
	}

	params := maintenances.NewPostAPIV1MaintenancesCreateParams().
		WithContext(ctx).
		WithRequest(req)

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesCreate(params)
	require.NoError(t, err, "Failed to create maintenance with resource")
	require.NotNil(t, resp, "Response should not be nil")

	return resp.Payload.ID.String()
}

func approveMaintenance(ctx context.Context, t *testing.T, apiClient *client.Maintmode, maintenanceID string) {
	t.Helper()

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams)
	require.NoError(t, err, "Failed to get maintenance before approve")
	require.NotNil(t, getResp, "Get response should not be nil")

	revision := time.Time(getResp.Payload.CreatedAt).UnixMicro()
	if !time.Time(getResp.Payload.UpdatedAt).IsZero() {
		revision = time.Time(getResp.Payload.UpdatedAt).UnixMicro()
	}

	approveReq := &models.ApimodelsApproveDraftMaintRequest{
		ObservedMaintRevision: revision,
		ConflictsSnapshot:     []*models.ApimodelsConflict{},
	}

	params := maintenances.NewPostAPIV1MaintenancesIDApproveParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID)).
		WithRequest(approveReq)

	_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDApprove(params)
	require.NoError(t, err, "Failed to approve maintenance")
}

func createAndApproveMaintenance(ctx context.Context, t *testing.T, apiClient *client.Maintmode) string {
	t.Helper()
	maintenanceID := createTestMaintenance(ctx, t, apiClient)
	approveMaintenance(ctx, t, apiClient, maintenanceID)
	return maintenanceID
}

func createAndStartMaintenance(ctx context.Context, t *testing.T, apiClient *client.Maintmode) string {
	t.Helper()

	maintenanceID := createAndApproveMaintenance(ctx, t, apiClient)

	params := maintenances.NewPostAPIV1MaintenancesIDStartParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	_, err := apiClient.Maintenances.PostAPIV1MaintenancesIDStart(params)
	require.NoError(t, err, "Failed to start maintenance")

	return maintenanceID
}

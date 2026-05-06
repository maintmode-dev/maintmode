package api

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
	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog/xfield"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	"github.com/ruko1202/maintmode/test/api/client/client"
	"github.com/ruko1202/maintmode/test/api/client/client/maintenances"
	"github.com/ruko1202/maintmode/test/api/client/client/resources"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

const (
	// Environment variables for test configuration
	envAPIHost     = "TEST_API_HOST"
	envAPIPort     = "TEST_API_PORT"
	envHealthCheck = "TEST_HEALTH_CHECK"

	// Default values
	defaultAPIHost = "localhost"
	defaultAPIPort = "9000"

	// Health check configuration
	healthCheckURL      = "http://localhost:9001/maintmode/readiness"
	healthCheckTimeout  = 10 * time.Second
	healthCheckInterval = 100 * time.Millisecond

	// Test maintenance timing constants
	testMaintenanceStartOffset = 24 * time.Hour
	testMaintenanceDuration    = 2 * time.Hour
	testMaintenanceLongOffset  = 48 * time.Hour
	testAPIJWTIssuer           = "oauth-service"
	testAPIJWTPrivateKey       = "1be2f1f68285c972b750b7718b00d5453f2c08f88c7894d1b9013f75a439de20"
	testAPIJWTKid              = "99d1e557df9b619fd046322c1e7f196e"
)

func TestMain(m *testing.M) {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Panic(err)
	}
	zapAdapter := xlog.NewZapAdapter(l)
	xlog.ReplaceGlobalLogger(zapAdapter)
	ctx := xlog.ContextWithLogger(context.Background(), zapAdapter)

	viper.AutomaticEnv()

	viper.SetDefault(envAPIHost, defaultAPIHost)
	viper.SetDefault(envAPIPort, defaultAPIPort)
	viper.SetDefault(envHealthCheck, false)

	if err := waitForAPIHealth(ctx); err != nil {
		xlog.Panic(ctx, "Failed to wait for API health check", xfield.Error(err))
	}

	code := m.Run()
	os.Exit(code)
}

func waitForAPIHealth(ctx context.Context) error {
	xlog.Info(ctx, "Waiting for API to become healthy", xfield.String("url", healthCheckURL))

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
			xlog.Info(ctx, "Ping service", xfield.String("url", healthCheckURL))
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthCheckURL, http.NoBody)
			if err != nil {
				xlog.Info(ctx, "Failed to create health check request", xfield.Error(err))
				continue
			}

			resp, err := httpClient.Do(req)
			if err != nil {
				xlog.Info(ctx, "Health check failed", xfield.Error(err))
				continue
			}

			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusNoContent {
				xlog.Info(ctx, "API is healthy", xfield.Int("status", resp.StatusCode))
				return nil
			}

			xlog.Info(ctx, "API not yet healthy", xfield.Int("status", resp.StatusCode))
		}
	}
}

func setupMaintmodeTestClient() *client.Maintmode {
	return setupMaintmodeTestClientWithRoles(entity.RoleAdmin)
}

func setupMaintmodeTestClientWithoutAuth() *client.Maintmode {
	return newMaintmodeTestClient("")
}

func setupMaintmodeTestClientWithRoles(roles ...entity.Role) *client.Maintmode {
	return newMaintmodeTestClient(mustTestAccessToken(roles...))
}

func setupMaintmodeTestClientWithToken(token string) *client.Maintmode {
	return newMaintmodeTestClient(token)
}

func newMaintmodeTestClient(token string) *client.Maintmode {
	transport := httptransport.New(
		fmt.Sprintf("%s:%s", viper.GetString(envAPIHost), viper.GetString(envAPIPort)),
		"/maintmode",
		[]string{"http"},
	)
	if token != "" {
		transport.DefaultAuthentication = httptransport.BearerToken(token)
	}
	return client.New(transport, strfmt.Default)
}

func setupAuthTestClientWithRoles(roles ...entity.Role) *client.Maintmode {
	transport := httptransport.New(
		fmt.Sprintf("%s:%s", viper.GetString(envAPIHost), viper.GetString(envAPIPort)),
		"/auth",
		[]string{"http"},
	)
	transport.DefaultAuthentication = httptransport.BearerToken(mustTestAccessToken(roles...))
	return client.New(transport, strfmt.Default)
}

func mustTestAccessToken(roles ...entity.Role) string {
	if len(roles) == 0 {
		roles = []entity.Role{entity.RoleAdmin}
	}

	key := config.JWT{PrivateKey: testAPIJWTPrivateKey}.GeneratePrivateKey()
	now := xtime.UTCNow()
	claims := entity.AccessClaims{
		Email: "api-test@example.com",
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        xuuid.NewString(),
			Subject:   xuuid.NewString(),
			Issuer:    testAPIJWTIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = testAPIJWTKid
	signed, err := token.SignedString(key)
	if err != nil {
		panic(fmt.Sprintf("sign test access token: %v", err))
	}

	return signed
}

func testMaintenanceSteps() []*models.ApimodelsMaintenanceStepInput {
	return []*models.ApimodelsMaintenanceStepInput{
		{
			Order:               1,
			Description:         "Run maintenance task",
			RollbackDescription: "Rollback maintenance task",
			Duration:            testMaintenanceDuration.String(),
		},
	}
}

func createTestMaintenance(ctx context.Context, t *testing.T, apiClient *client.Maintmode) string {
	t.Helper()

	resource := creatResource(ctx, t, apiClient)

	now := xtime.UTCNow()
	plannedStart := strfmt.DateTime(now.Add(testMaintenanceStartOffset))

	req := &models.ApimodelsCreateDraftMaintRequest{
		Title:        "Test Maintenance",
		Description:  "Test maintenance for API testing",
		Impact:       models.ApimodelsMaintenanceImpactNone,
		Scope:        models.ApimodelsMaintenanceScopeResource,
		PlannedStart: plannedStart,
		Resources: []*models.ApimodelsResource{
			{
				ID:   strfmt.UUID(resource.ID),
				Type: models.ApimodelsResourceTypeDatabase,
			},
		},
		Steps: testMaintenanceSteps(),
	}

	params := maintenances.NewPostAPIV1MaintenancesCreateParams().
		WithContext(ctx).
		WithRequest(req)

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesCreate(params, nil)
	require.NoError(t, err, "Failed to create test maintenance")
	require.NotNil(t, resp, "Response should not be nil")

	return resp.Payload.ID.String()
}

func createMaintenanceWithResource(ctx context.Context, t *testing.T, apiClient *client.Maintmode, resourceID string) string {
	t.Helper()

	resourceUUID := strfmt.UUID(resourceID)

	now := xtime.UTCNow()
	plannedStart := strfmt.DateTime(now.Add(testMaintenanceStartOffset))

	req := &models.ApimodelsCreateDraftMaintRequest{
		Title:        "Test Maintenance with Resource",
		Description:  "Maintenance for testing resource-based operations",
		Impact:       models.ApimodelsMaintenanceImpactNone,
		Scope:        models.ApimodelsMaintenanceScopeResource,
		PlannedStart: plannedStart,
		Resources: []*models.ApimodelsResource{
			{
				ID:   resourceUUID,
				Type: models.ApimodelsResourceTypeService,
			},
		},
		Steps: testMaintenanceSteps(),
	}

	params := maintenances.NewPostAPIV1MaintenancesCreateParams().
		WithContext(ctx).
		WithRequest(req)

	resp, err := apiClient.Maintenances.PostAPIV1MaintenancesCreate(params, nil)
	require.NoError(t, err, "Failed to create maintenance with resource")
	require.NotNil(t, resp, "Response should not be nil")

	return resp.Payload.ID.String()
}

func approveMaintenance(ctx context.Context, t *testing.T, apiClient *client.Maintmode, maintenanceID string) {
	t.Helper()

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams, nil)
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

	_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDApprove(params, nil)
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

	_, err := apiClient.Maintenances.PostAPIV1MaintenancesIDStart(params, nil)
	require.NoError(t, err, "Failed to start maintenance")

	return maintenanceID
}

func completeMaintenanceSteps(ctx context.Context, t *testing.T, apiClient *client.Maintmode, maintenanceID string) {
	t.Helper()

	getParams := maintenances.NewGetAPIV1MaintenancesIDParams().
		WithContext(ctx).
		WithID(strfmt.UUID(maintenanceID))

	getResp, err := apiClient.Maintenances.GetAPIV1MaintenancesID(getParams, nil)
	require.NoError(t, err, "Failed to get maintenance steps")
	require.NotNil(t, getResp, "Get response should not be nil")
	require.NotEmpty(t, getResp.Payload.Steps, "Maintenance should have steps")

	for _, step := range getResp.Payload.Steps {
		startParams := maintenances.NewPostAPIV1MaintenancesIDStepsStepIDStartParams().
			WithContext(ctx).
			WithID(strfmt.UUID(maintenanceID)).
			WithStepID(step.ID)

		_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDStepsStepIDStart(startParams, nil)
		require.NoError(t, err, "Failed to start maintenance step")

		completeParams := maintenances.NewPostAPIV1MaintenancesIDStepsStepIDCompleteParams().
			WithContext(ctx).
			WithID(strfmt.UUID(maintenanceID)).
			WithStepID(step.ID)

		_, err = apiClient.Maintenances.PostAPIV1MaintenancesIDStepsStepIDComplete(completeParams, nil)
		require.NoError(t, err, "Failed to complete maintenance step")
	}
}

func creatResource(ctx context.Context, t *testing.T, apiClient *client.Maintmode) *models.ApismodelsResource {
	t.Helper()

	req := &models.ApismodelsCreateResourceRequest{
		Name:        fmt.Sprintf("test name: %s [%s]", t.Name(), xuuid.NewString()),
		Description: "This is a test resource created via API tests",
		ExternalID:  xuuid.NewString(),
	}

	params := resources.NewPostAPIV1ResourceCreateParams().
		WithContext(ctx).
		WithRequest(req)

	resp, err := apiClient.Resources.PostAPIV1ResourceCreate(params, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	return resp.Payload
}

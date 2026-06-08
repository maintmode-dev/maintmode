package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
	maintmodeclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/maintmode"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
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
	healthCheckTimeout  = 3 * time.Minute
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

//nolint:unparam,prealloc // by design
func ctxWithLogger(ctx context.Context, t *testing.T, option ...zaptest.LoggerOption) context.Context {
	t.Helper()

	opts := []zaptest.LoggerOption{zaptest.Level(zap.InfoLevel)}
	opts = append(opts, option...)

	return xlog.ContextWithLogger(ctx, xlog.NewZapAdapter(
		zaptest.NewLogger(t, opts...),
	))
}

func waitForAPIHealth(ctx context.Context) error {
	xlog.Info(ctx, "Waiting for API to become healthy", xfield.String("url", healthCheckURL))

	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	httpClient := xhttp.NewClient(xhttp.WithTimeout(5 * time.Second))

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

// baseURL builds the proxy base URL for a service prefix (e.g. /maintmode, /auth).
func baseURL(servicePrefix string) string {
	return fmt.Sprintf("http://%s:%s/%s",
		viper.GetString(envAPIHost), viper.GetString(envAPIPort), servicePrefix)
}

// bearerEditor returns a RequestEditorFn that attaches the Bearer token to every
// request, or a no-op editor when the token is empty (unauthenticated client).
func bearerEditor(token string) maintmodeclient.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		if token != "" {
			xhttp.SetBearerToken(req, token)
		}
		return nil
	}
}

// --- maintmode service client ---------------------------------------------

func setupMaintmodeTestClient() *maintmodeclient.ClientWithResponses {
	return setupMaintmodeTestClientWithRoles(entity.RoleAdmin)
}

func setupMaintmodeTestClientWithoutAuth() *maintmodeclient.ClientWithResponses {
	return newMaintmodeTestClient("")
}

func setupMaintmodeTestClientWithRoles(roles ...entity.Role) *maintmodeclient.ClientWithResponses {
	return newMaintmodeTestClient(mustTestAccessToken(roles...))
}

func setupMaintmodeTestClientWithToken(token string) *maintmodeclient.ClientWithResponses {
	return newMaintmodeTestClient(token)
}

func newMaintmodeTestClient(token string) *maintmodeclient.ClientWithResponses {
	c, err := maintmodeclient.NewClientWithResponses(
		baseURL("maintmode"),
		maintmodeclient.WithHTTPClient(xhttp.NewClient()),
		maintmodeclient.WithRequestEditorFn(bearerEditor(token)),
	)
	if err != nil {
		panic(fmt.Sprintf("build maintmode test client: %v", err))
	}
	return c
}

// --- auth service client ---------------------------------------------------

func setupAuthTestClientWithRoles(roles ...entity.Role) *authclient.ClientWithResponses {
	return newAuthTestClient(mustTestAccessToken(roles...))
}

func newAuthTestClient(token string) *authclient.ClientWithResponses {
	c, err := authclient.NewClientWithResponses(
		baseURL("auth"),
		authclient.WithHTTPClient(xhttp.NewClient()),
		authclient.WithRequestEditorFn(authclient.RequestEditorFn(bearerEditor(token))),
	)
	if err != nil {
		panic(fmt.Sprintf("build auth test client: %v", err))
	}
	return c
}

func mustTestAccessToken(roles ...entity.Role) string {
	return mustTestAccessTokenForUser(xuuid.NewString(), roles...)
}

// mustTestAccessTokenForUser mints a signed access token for a specific subject
// (user id). Use it when the acting user's identity matters downstream — e.g.
// approving a maintenance, where the actor must be the assigned approver.
func mustTestAccessTokenForUser(subject string, roles ...entity.Role) string {
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
			Subject:   subject,
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

func testMaintenanceSteps() []maintmodeclient.ApimodelsMaintenanceStepInput {
	return []maintmodeclient.ApimodelsMaintenanceStepInput{
		{
			Order:               lo.ToPtr(1),
			Description:         lo.ToPtr("Run maintenance task"),
			RollbackDescription: lo.ToPtr("Rollback maintenance task"),
			Duration:            lo.ToPtr(testMaintenanceDuration.String()),
		},
	}
}

func createTestMaintenance(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) string {
	t.Helper()

	resource := creatResource(ctx, t, apiClient)

	plannedStart := xtime.UTCNow().Add(testMaintenanceStartOffset)

	req := maintmodeclient.PostApiV1MaintenancesCreateJSONRequestBody{
		Title:        lo.ToPtr("Test Maintenance"),
		Description:  lo.ToPtr("Test maintenance for API testing"),
		Impact:       lo.ToPtr(maintmodeclient.MaintenanceImpactNone),
		Scope:        lo.ToPtr(maintmodeclient.MaintenanceScopeResources),
		PlannedStart: lo.ToPtr(plannedStart),
		Resources: lo.ToPtr([]maintmodeclient.ApimodelsResourceRef{
			{Id: lo.ToPtr(uuid.MustParse(lo.FromPtr(resource.Id)))},
		}),
		Steps:          lo.ToPtr(testMaintenanceSteps()),
		NotifyTargets:  testNotifyTargets(ctx, t, apiClient),
		ApproverUserId: lo.ToPtr(resolveEligibleApprover(ctx, t, apiClient)),
	}

	resp, err := apiClient.PostApiV1MaintenancesCreateWithResponse(ctx, req)
	require.NoError(t, err, "Failed to create test maintenance")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	return lo.FromPtr(resp.JSON200.Id).String()
}

func createMaintenanceWithResource(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses, resourceID string) string {
	t.Helper()

	resourceUUID := uuid.MustParse(resourceID)
	plannedStart := xtime.UTCNow().Add(testMaintenanceStartOffset)

	req := maintmodeclient.PostApiV1MaintenancesCreateJSONRequestBody{
		Title:        lo.ToPtr("Test Maintenance with Resource"),
		Description:  lo.ToPtr("Maintenance for testing resource-based operations"),
		Impact:       lo.ToPtr(maintmodeclient.MaintenanceImpactNone),
		Scope:        lo.ToPtr(maintmodeclient.MaintenanceScopeResources),
		PlannedStart: lo.ToPtr(plannedStart),
		Resources: lo.ToPtr([]maintmodeclient.ApimodelsResourceRef{
			{Id: lo.ToPtr(resourceUUID)},
		}),
		Steps:          lo.ToPtr(testMaintenanceSteps()),
		NotifyTargets:  testNotifyTargets(ctx, t, apiClient),
		ApproverUserId: lo.ToPtr(resolveEligibleApprover(ctx, t, apiClient)),
	}

	resp, err := apiClient.PostApiV1MaintenancesCreateWithResponse(ctx, req)
	require.NoError(t, err, "Failed to create maintenance with resource")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	return lo.FromPtr(resp.JSON200.Id).String()
}

// resolveEligibleApprover returns the id of an active reviewer/admin from the
// auth service via the assignable-users picker (the same path the UI uses).
// Approver assignment is validated at write time against this set, so tests must
// pick a real eligible user rather than a random uuid.
func resolveEligibleApprover(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) uuid.UUID {
	t.Helper()

	resp, err := apiClient.GetApiV1UsersAssignableWithResponse(ctx, &maintmodeclient.GetApiV1UsersAssignableParams{
		Roles: lo.ToPtr([]string{string(entity.RoleReviewer), string(entity.RoleAdmin)}),
		Limit: lo.ToPtr(1),
	})
	require.NoError(t, err, "Failed to list assignable users")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)
	require.NotEmpty(t, lo.FromPtr(resp.JSON200.Users), "no eligible (reviewer/admin) approver in auth")

	id, err := uuid.Parse(lo.FromPtr(lo.FromPtr(resp.JSON200.Users)[0].Id))
	require.NoError(t, err, "malformed approver id")
	return id
}

func approveMaintenance(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses, maintenanceID string) {
	t.Helper()

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance before approve")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)

	revision := int(lo.FromPtr(getResp.JSON200.CreatedAt).UnixMicro())
	if updated := lo.FromPtr(getResp.JSON200.UpdatedAt); !updated.IsZero() {
		revision = int(updated.UnixMicro())
	}

	approveReq := maintmodeclient.PostApiV1MaintenancesIdApproveJSONRequestBody{
		ObservedMaintRevision: lo.ToPtr(revision),
		ConflictsSnapshot:     lo.ToPtr([]maintmodeclient.ApimodelsConflict{}),
	}

	// Only the assigned approver may approve, so act as that user: mint a token
	// whose subject is the maintenance's approver id.
	approverID := lo.FromPtr(lo.FromPtr(getResp.JSON200.Approver).Id)
	approverClient := setupMaintmodeTestClientWithToken(
		mustTestAccessTokenForUser(approverID.String(), entity.RoleReviewer),
	)

	resp, err := approverClient.PostApiV1MaintenancesIdApproveWithResponse(ctx, uuid.MustParse(maintenanceID), approveReq)
	require.NoError(t, err, "Failed to approve maintenance")
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), "unexpected status: %s", resp.Body)
}

func createAndApproveMaintenance(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) string {
	t.Helper()
	maintenanceID := createTestMaintenance(ctx, t, apiClient)
	approveMaintenance(ctx, t, apiClient, maintenanceID)
	return maintenanceID
}

func createAndStartMaintenance(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) string {
	t.Helper()

	maintenanceID := createAndApproveMaintenance(ctx, t, apiClient)

	resp, err := apiClient.PostApiV1MaintenancesIdStartWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to start maintenance")
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), "unexpected status: %s", resp.Body)

	return maintenanceID
}

func completeMaintenanceSteps(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses, maintenanceID string) {
	t.Helper()

	getResp, err := apiClient.GetApiV1MaintenancesIdWithResponse(ctx, uuid.MustParse(maintenanceID))
	require.NoError(t, err, "Failed to get maintenance steps")
	require.Equal(t, http.StatusOK, getResp.StatusCode(), "unexpected status: %s", getResp.Body)
	require.NotNil(t, getResp.JSON200)
	require.NotEmpty(t, lo.FromPtr(getResp.JSON200.Steps), "Maintenance should have steps")

	for _, step := range lo.FromPtr(getResp.JSON200.Steps) {
		stepID := lo.FromPtr(step.Id)

		startResp, err := apiClient.PostApiV1MaintenancesIdStepsStepIdStartWithResponse(ctx, uuid.MustParse(maintenanceID), stepID)
		require.NoError(t, err, "Failed to start maintenance step")
		require.Equal(t, http.StatusNoContent, startResp.StatusCode(), "unexpected status: %s", startResp.Body)

		completeResp, err := apiClient.PostApiV1MaintenancesIdStepsStepIdCompleteWithResponse(ctx, uuid.MustParse(maintenanceID), stepID)
		require.NoError(t, err, "Failed to complete maintenance step")
		require.Equal(t, http.StatusNoContent, completeResp.StatusCode(), "unexpected status: %s", completeResp.Body)
	}
}

func creatResource(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) maintmodeclient.ApimodelsResource {
	t.Helper()

	req := maintmodeclient.PostApiV1ResourceCreateJSONRequestBody{
		Name:        lo.ToPtr(fmt.Sprintf("test name: %s [%s]", t.Name(), xuuid.NewString())),
		Description: lo.ToPtr("This is a test resource created via API tests"),
		ExternalId:  lo.ToPtr(xuuid.NewString()),
	}

	resp, err := apiClient.PostApiV1ResourceCreateWithResponse(ctx, req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	return *resp.JSON200
}

// testNotifyTargets returns a NotifyTargets payload referencing the
// first channel from the live catalog. Notify targets are required on
// maintenance creation, so every create helper attaches this. The
// catalog now lives in Postgres, so ensure at least one channel exists
// (creating one via the admin API on the first call).
func testNotifyTargets(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) *maintmodeclient.ApimodelsNotifyTargets {
	t.Helper()

	channelID := ensureNotifyChannel(ctx, t, apiClient)

	return &maintmodeclient.ApimodelsNotifyTargets{
		ChannelIds: lo.ToPtr([]string{channelID}),
	}
}

// createNotifyChannel always creates a fresh catalog channel and returns
// its id. Use when a test needs a channel isolated from any other
// maintenance (e.g. filtering the calendar by a single channel).
func createNotifyChannel(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) string {
	t.Helper()

	createResp, err := apiClient.PostApiV1NotificationsChannelsWithResponse(ctx,
		maintmodeclient.PostApiV1NotificationsChannelsJSONRequestBody{
			Transport:          lo.ToPtr(string(entity.NotifyTransportSlack)),
			TransportChannelId: lo.ToPtr("api-test-" + xuuid.NewString()),
			Name:               lo.ToPtr("API test channel"),
			Description:        lo.ToPtr("Seeded by API integration tests"),
		},
	)
	require.NoError(t, err, "Failed to create notification channel")
	require.Equal(t, http.StatusCreated, createResp.StatusCode(), "unexpected status: %s", createResp.Body)
	require.NotNil(t, createResp.JSON201)

	return lo.FromPtr(createResp.JSON201.Id).String()
}

// createMaintenanceWithChannel creates a maintenance whose only notify
// target is the given channel, returning the maintenance id.
func createMaintenanceWithChannel(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses, channelID string) string {
	t.Helper()

	resource := creatResource(ctx, t, apiClient)
	plannedStart := xtime.UTCNow().Add(testMaintenanceStartOffset)

	req := maintmodeclient.PostApiV1MaintenancesCreateJSONRequestBody{
		Title:        lo.ToPtr("Test Maintenance with Channel"),
		Description:  lo.ToPtr("Maintenance for testing channel-based filtering"),
		Impact:       lo.ToPtr(maintmodeclient.MaintenanceImpactNone),
		Scope:        lo.ToPtr(maintmodeclient.MaintenanceScopeResources),
		PlannedStart: lo.ToPtr(plannedStart),
		Resources: lo.ToPtr([]maintmodeclient.ApimodelsResourceRef{
			{Id: lo.ToPtr(uuid.MustParse(lo.FromPtr(resource.Id)))},
		}),
		Steps:          lo.ToPtr(testMaintenanceSteps()),
		NotifyTargets:  &maintmodeclient.ApimodelsNotifyTargets{ChannelIds: lo.ToPtr([]string{channelID})},
		ApproverUserId: lo.ToPtr(resolveEligibleApprover(ctx, t, apiClient)),
	}

	resp, err := apiClient.PostApiV1MaintenancesCreateWithResponse(ctx, req)
	require.NoError(t, err, "Failed to create maintenance with channel")
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	return lo.FromPtr(resp.JSON200.Id).String()
}

// ensureNotifyChannel returns the id of a catalog channel, creating one
// via the admin API if the catalog is empty. Channel creation is
// admin-scoped; the default API test client carries the admin role.
func ensureNotifyChannel(ctx context.Context, t *testing.T, apiClient *maintmodeclient.ClientWithResponses) string {
	t.Helper()

	listResp, err := apiClient.GetApiV1NotificationsChannelsWithResponse(ctx, nil)
	require.NoError(t, err, "Failed to fetch notification channels")
	require.Equal(t, http.StatusOK, listResp.StatusCode(), "unexpected status: %s", listResp.Body)
	require.NotNil(t, listResp.JSON200)

	if channels := lo.FromPtr(listResp.JSON200.Channels); len(channels) > 0 {
		return lo.FromPtr(channels[0].Id).String()
	}

	return createNotifyChannel(ctx, t, apiClient)
}

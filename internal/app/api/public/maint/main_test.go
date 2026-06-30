package apimaint

import (
	"context"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	redisDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"

	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	apinotifications "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets"
	notificationsmodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
	resourcesapi "github.com/ruko1202/maintmode/internal/app/api/public/resources"
	resourcemodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

var (
	db                   *sqlx.DB
	redis                *redisDB.Client
	cfg                  *config.AppConfig
	testMaintenanceIndex atomic.Int64
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	redis = testdbconnutils.NewRedisClient(cfg)
	closer.Add(redis.Close)

	code := m.Run()

	os.Exit(code)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	services := testbootstraputils.InitServicesT(context.Background(), t, db, redis, cfg)

	return New(services.Maint, services.UserSummary)
}

func createDraftMaintenance(ctx context.Context, t *testing.T, impl *Implementation) *apimodels.CreateDraftMaintResponse {
	t.Helper()

	notifyChan := makeNotifyChannel(ctx, t)
	resource := createResource(ctx, t)
	plannedStart := time.Now().
		AddDate(100, 0, 0).
		Add(time.Duration(testMaintenanceIndex.Add(1)) * 2 * time.Hour)
	req := &apimodels.CreateDraftMaintRequest{
		Title:        "Test maintenance " + uuid.New().String()[:8],
		Description:  "Test description",
		PlannedStart: plannedStart,
		Scope:        apimodels.MaintenanceScopeResources,
		Impact:       apimodels.MaintenanceImpactPartial,
		Resources: []*apimodels.ResourceRef{
			resource,
		},
		Steps: []*apimodels.MaintenanceStepInput{
			{
				Order:               1,
				Description:         "Step 1",
				RollbackDescription: "Rollback step 1",
				Duration:            "30m",
			},
		},
		NotifyTargets: &apimodels.NotifyTargets{
			ChannelIDs: []string{notifyChan.ID},
		},
		ApproverUserID: seedApprover(ctx, t),
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	xecho.UserToEchoCtx(c, makeUser(t))

	err := impl.CreateDraftMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	resp := testjsonudils.JSONToAny[apimodels.CreateDraftMaintResponse](t, rec.Body)
	return &resp
}

// seedApprover provisions a real, persisted, approver-eligible (reviewer) user
// and returns its id. CreateDraftMaint validates the approver against the real
// user backend, so the assigned approver must be a real eligible user rather
// than a random uuid.
func seedApprover(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()

	services := testbootstraputils.InitServicesT(ctx, t, db, redis, cfg)
	return testbootstraputils.SeedEligibleApprover(ctx, t, services).ID
}

// makeUser builds an authenticated user to act as the maintenance author.
// CreateDraftMaint reads the author from the Echo context (mirroring what the
// auth middleware sets from the access token), so handler tests must put a user
// there. No DB row is needed: created_by_user_id has no FK to users.
func makeUser(t *testing.T) *entity.User {
	t.Helper()

	return &entity.User{
		ID:    uuid.New(),
		Email: "author-" + uuid.NewString() + "@example.com",
		Name:  "Author " + t.Name(),
		Roles: entity.DefaultRoles,
	}
}

func approveMaint(t *testing.T, impl *Implementation, maint *apimodels.CreateDraftMaintResponse) {
	t.Helper()

	approveReq := &apimodels.ApproveDraftMaintRequest{
		ObservedMaintRevision: maint.CreatedAt.UnixMicro(),
	}
	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, approveReq),
	}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: maint.ID.String()},
	})
	// Only the assigned approver may approve, so act as that user.
	xecho.UserToEchoCtx(c, approverUser(maint))

	err := impl.ApproveMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}

// approverUser builds the authenticated user that the maintenance was assigned
// to as approver, so an approve call passes the assigned-approver guard.
func approverUser(maint *apimodels.CreateDraftMaintResponse) *entity.User {
	return &entity.User{
		ID:    maint.ApproverUserID.ID,
		Email: "approver-" + maint.ApproverUserID.ID.String() + "@example.com",
		Name:  "Approver",
		Roles: []entity.Role{entity.RoleReviewer},
	}
}

func startMaint(t *testing.T, impl *Implementation, maint *apimodels.CreateDraftMaintResponse) {
	t.Helper()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: maint.ID.String()},
	})
	xecho.UserToEchoCtx(c, makeUser(t))

	err := impl.StartMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
}

func startStep(t *testing.T, impl *Implementation, maintID, stepID uuid.UUID) {
	t.Helper()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: maintID.String()},
		{Name: "step_id", Value: stepID.String()},
	})
	xecho.UserToEchoCtx(c, makeUser(t))

	err := impl.StartStep(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func getMaintByID(t *testing.T, impl *Implementation, id uuid.UUID) *apimodels.Maintenance {
	t.Helper()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: id.String()},
	})

	err := impl.GetMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	maint := testjsonudils.JSONToAny[apimodels.Maintenance](t, rec.Body)
	return &maint
}

func requireMaintStillMatchesDraft(t *testing.T, draft *apimodels.CreateDraftMaintResponse, maint *apimodels.Maintenance) {
	t.Helper()

	require.Equal(t, draft.ID, maint.ID)
	require.Equal(t, draft.Title, maint.Title)
	require.Equal(t, draft.Description, maint.Description)
	require.Equal(t, draft.PlannedPeriod, maint.PlannedPeriod)
	require.Equal(t, apimodels.MaintenanceScope(draft.Scope), maint.Scope)
	require.Equal(t, apimodels.MaintenanceImpact(draft.Impact), maint.Impact)
	require.Equal(t, draft.CreatedAt.UnixMicro(), maint.CreatedAt.UnixMicro())
	require.Equal(t, draft.Resources, maint.Resources)
	require.Len(t, maint.Steps, len(draft.Steps))
	for idx := range draft.Steps {
		require.Equal(t, draft.Steps[idx].ID, maint.Steps[idx].ID)
		require.Equal(t, draft.Steps[idx].Order, maint.Steps[idx].Order)
		require.Equal(t, draft.Steps[idx].Description, maint.Steps[idx].Description)
		require.Equal(t, draft.Steps[idx].RollbackDescription, maint.Steps[idx].RollbackDescription)
		require.Equal(t, draft.Steps[idx].DurationMinutes, maint.Steps[idx].DurationMinutes)
	}
}

func createResource(ctx context.Context, t *testing.T) *apimodels.ResourceRef {
	t.Helper()

	services := testbootstraputils.InitServicesT(context.Background(), t, db, redis, cfg)

	impl := resourcesapi.New(services.Resources, services.UserSummary)
	req := &resourcemodels.CreateResourceRequest{
		Name:        "test-resource-" + uuid.New().String(),
		Description: "Test resource",
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))
	// CreateResource captures the author from the Echo context (set by the auth
	// middleware in production), so the setup helper must seed a user.
	xecho.UserToEchoCtx(c, &entity.User{ID: uuid.New(), Name: t.Name(), Email: t.Name() + "@example.com"})

	err := impl.CreateResource(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	resource := testjsonudils.JSONToAny[resourcemodels.Resource](t, rec.Body)

	return &apimodels.ResourceRef{
		ID: resource.ID,
	}
}

// makeNotifyChannel seeds one catalog channel via the admin create endpoint
// and returns it. The catalog now lives in Postgres, so tests must create the
// channel they reference rather than relying on config-seeded stubs.
func makeNotifyChannel(ctx context.Context, t *testing.T) *notificationsmodels.Channel {
	t.Helper()

	services := testbootstraputils.InitServicesT(context.Background(), t, db, redis, cfg)

	impl := apinotifications.New(services.NotifyTargets, services.UserSummary)

	req := &notificationsmodels.CreateChannelRequest{
		Transport:          string(entity.NotifyTransportTelegram),
		TransportChannelID: t.Name() + "-" + uuid.NewString(),
		Name:               t.Name(),
		Description:        "test channel",
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))
	xecho.UserToEchoCtx(c, &entity.User{ID: uuid.New(), Name: t.Name(), Email: t.Name() + "@example.com"})

	err := impl.CreateChannel(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, rec.Code)

	return testjsonudils.JSONToAny[*notificationsmodels.Channel](t, rec.Body)
}

// createTwoStepDraftMaintenance is createDraftMaintenance with a second step,
// so step-order scenarios (e.g. starting step 2 while step 1 is still pending)
// can be exercised. Steps come back ordered by step_order ASC, so the response
// Steps[0]/Steps[1] are order 1/2 respectively.
func createTwoStepDraftMaintenance(ctx context.Context, t *testing.T, impl *Implementation) *apimodels.CreateDraftMaintResponse {
	t.Helper()

	notifyChan := makeNotifyChannel(ctx, t)
	resource := createResource(ctx, t)
	plannedStart := time.Now().
		AddDate(100, 0, 0).
		Add(time.Duration(testMaintenanceIndex.Add(1)) * 2 * time.Hour)
	req := &apimodels.CreateDraftMaintRequest{
		Title:        "Test maintenance " + uuid.New().String()[:8],
		Description:  "Test description",
		PlannedStart: plannedStart,
		Scope:        apimodels.MaintenanceScopeResources,
		Impact:       apimodels.MaintenanceImpactPartial,
		Resources: []*apimodels.ResourceRef{
			resource,
		},
		Steps: []*apimodels.MaintenanceStepInput{
			{
				Order:               1,
				Description:         "Step 1",
				RollbackDescription: "Rollback step 1",
				Duration:            "30m",
			},
			{
				Order:               2,
				Description:         "Step 2",
				RollbackDescription: "Rollback step 2",
				Duration:            "30m",
			},
		},
		NotifyTargets: &apimodels.NotifyTargets{
			ChannelIDs: []string{notifyChan.ID},
		},
		ApproverUserID: seedApprover(ctx, t),
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	xecho.UserToEchoCtx(c, makeUser(t))

	err := impl.CreateDraftMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	resp := testjsonudils.JSONToAny[apimodels.CreateDraftMaintResponse](t, rec.Body)
	return &resp
}

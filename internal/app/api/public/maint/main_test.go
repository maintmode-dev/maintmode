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
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	resourcesapi "github.com/ruko1202/maintmode/internal/app/api/public/resources"
	resourcemodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"
)

var (
	db                   *sqlx.DB
	cfg                  *config.AppConfig
	testMaintenanceIndex atomic.Int64
)

func TestMain(m *testing.M) {
	cfg = config.LoadAppConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

const maintmodePolicy = `
g, editor, guest
g, reviewer, editor
g, admin, reviewer

p, guest, calendar.read, execute
p, guest, maintenance.read, execute
p, guest, resource.read, execute

p, editor, maintenance.create, execute
p, editor, maintenance.edit, execute
p, editor, maintenance.start, execute
p, editor, maintenance.complete, execute
p, editor, maintenance.cancel, execute
p, editor, maintenance.step.start, execute
p, editor, maintenance.step.complete, execute
p, editor, maintenance.step.cancel, execute
p, editor, resource.create, execute

p, reviewer, maintenance.approve, execute
`

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	cfg.RBAC = config.RbacConfig{
		ModelPath:  "../../../../../deployment/maintmode/authz/model.conf",
		Adapter:    config.AuthorizationAdapterMemory,
		PolicyData: maintmodePolicy,
	}

	stores := bootstrap.NewStores(db)
	services, err := bootstrap.NewServices(context.Background(), cfg, stores)
	require.NoError(t, err)

	return New(services.Maint)
}

func createDraftMaintenance(ctx context.Context, t *testing.T, impl *Implementation) *apimodels.CreateDraftMaintResponse {
	t.Helper()

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
		Resources: []*apimodels.Resource{
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
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)

	err := impl.CreateDraftMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testjsonudils.JSONToAny[apimodels.CreateDraftMaintResponse](t, rec.Body)
	return &resp
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

	err := impl.ApproveMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func startMaint(t *testing.T, impl *Implementation, maint *apimodels.CreateDraftMaintResponse) {
	t.Helper()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: maint.ID.String()},
	})

	err := impl.StartMaint(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func startStep(t *testing.T, impl *Implementation, maintID, stepID uuid.UUID) {
	t.Helper()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.SetPathValues(echo.PathValues{
		{Name: "id", Value: maintID.String()},
		{Name: "step_id", Value: stepID.String()},
	})

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

func createResource(ctx context.Context, t *testing.T) *apimodels.Resource {
	t.Helper()

	stores := bootstrap.NewStores(db)
	services, err := bootstrap.NewServices(context.Background(), cfg, stores)
	require.NoError(t, err)

	impl := resourcesapi.New(services.Resources)
	req := &resourcemodels.CreateResourceRequest{
		Name:        "test-resource-" + uuid.New().String(),
		Description: "Test resource",
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, req),
	}.ToContextRecorder(t)
	c.SetRequest(c.Request().WithContext(ctx))

	err = impl.CreateResource(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	resource := testjsonudils.JSONToAny[resourcemodels.Resource](t, rec.Body)

	return &apimodels.Resource{
		ID:   resource.ID,
		Type: apimodels.ResourceTypeService,
	}
}

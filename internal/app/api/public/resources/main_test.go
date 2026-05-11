package resourcesapi

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db  *sqlx.DB
	cfg *config.AppConfig
)

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

func TestMain(m *testing.M) {
	cfg = config.LoadAppConfig()
	cfg.RBAC = config.RbacConfig{
		ModelPath:  "../../../../../deployment/maintmode/authz/model.conf",
		Adapter:    config.AuthorizationAdapterMemory,
		PolicyData: maintmodePolicy,
	}

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	stores := bootstrap.NewStores(db)
	services, err := bootstrap.NewServices(context.Background(), cfg, stores)
	require.NoError(t, err)

	return New(services.Resources)
}

func makeResource(t *testing.T, impl *Implementation) *apimodels.Resource {
	t.Helper()

	createReq := &apimodels.CreateResourceRequest{
		Name:        "resource" + t.Name(),
		Description: t.Name(),
	}

	c, rec := echotest.ContextConfig{
		JSONBody: testjsonudils.AnyToJSONBytes(t, createReq),
	}.ToContextRecorder(t)

	err := impl.CreateResource(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	resource := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)

	return &resource
}

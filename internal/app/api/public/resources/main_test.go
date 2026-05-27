package resourcesapi

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db  *sqlx.DB
	cfg *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	services := testbootstraputils.InitServicesT(context.Background(), t, db, cfg)

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

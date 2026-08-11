package resourcesapi

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db     *sqlx.DB
	valkey *valkeyDB.Client
	cfg    *config.AppConfig
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	valkey = testdbconnutils.NewValkeyClient(cfg)
	closer.Add(valkey.Close)

	code := m.Run()

	os.Exit(code)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	services := testbootstraputils.InitServicesT(context.Background(), t, db, valkey, cfg)

	return New(services.Resources, services.UserSummary)
}

// seedUser puts an authenticated user on the Echo context, mimicking what the
// auth middleware does in production. The create/update handlers read the author
// / editor from there, so unit tests that call them directly must seed one.
func seedUser(t *testing.T, c *echo.Context) *entity.User {
	t.Helper()

	user := &entity.User{
		ID:    uuid.New(),
		Name:  "Author " + t.Name(),
		Email: "author-" + uuid.NewString() + "@example.com",
		Roles: entity.DefaultRoles,
	}
	xecho.UserToEchoCtx(c, user)

	return user
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
	seedUser(t, c)

	err := impl.CreateResource(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	resource := testjsonudils.JSONToAny[apimodels.Resource](t, rec.Body)

	return &resource
}

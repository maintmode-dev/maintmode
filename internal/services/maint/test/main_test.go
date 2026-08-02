package test

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	redisDB "github.com/redis/go-redis/v9"

	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

// testActor builds an authenticated user to stand in as the audit actor on
// mutations called directly against the service (the handler resolves it in
// production). A fresh id per call keeps shared-DB test data unique.
func testActor(roles ...entity.Role) *entity.User {
	return &entity.User{ID: uuid.New(), Email: "actor@example.com", Name: "Actor", Roles: roles}
}

var (
	db             *sqlx.DB
	redis          *redisDB.Client
	cfg            *config.AppConfig
	maintStore     *maintenances.Store
	resourcesStore *resources.Store
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	redis = testdbconnutils.NewRedisClient(cfg)
	closer.Add(redis.Close)

	stores := testbootstraputils.InitStores(db, redis)
	resourcesStore = stores.Resources
	maintStore = stores.Maintenances

	code := m.Run()

	os.Exit(code)
}

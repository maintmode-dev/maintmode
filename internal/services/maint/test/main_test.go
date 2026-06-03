package test

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/app/bootstrap"
	testbootstraputils "github.com/ruko1202/maintmode/test/utils/bootstrap"
	testconfigutils "github.com/ruko1202/maintmode/test/utils/config"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/services/conflicts"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db             *sqlx.DB
	cfg            *config.AppConfig
	services       *bootstrap.Services
	maintStore     *maintenances.Store
	resourcesStore *resources.Store
	conflictsSrv   *conflicts.Service
)

func TestMain(m *testing.M) {
	cfg = testconfigutils.LoadMaintConfig()

	db = testdbconnutils.NewDB(cfg)
	closer.Add(db.Close)

	stores := testbootstraputils.InitStores(db)
	resourcesStore = stores.Resources
	maintStore = stores.Maintenances

	services = testbootstraputils.InitServices(context.Background(), db, cfg)
	conflictsSrv = services.Conflicts

	code := m.Run()

	os.Exit(code)
}

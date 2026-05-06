package maint

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db             *sqlx.DB
	resourcesStore *resources.Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	resourcesStore = resources.NewStore(db)

	code := m.Run()

	os.Exit(code)
}

func initService(db *sqlx.DB) *Service {
	return NewService(
		dbtx.NewTxManager(db),
		maintenances.NewStore(db),
		resourcesStore,
		nil,
	)
}

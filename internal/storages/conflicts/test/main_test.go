package test

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db             *sqlx.DB
	maintsStore    *maintenances.Store
	resourcesStore *resources.Store
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	xlog.ReplaceGlobalLogger(xlog.NewZapAdapter(logger))

	conn := testdbconnutils.NewDB()
	db = conn
	closer.Add(conn.Close)

	maintsStore = maintenances.NewStore(conn)
	resourcesStore = resources.NewStore(conn)

	code := m.Run()

	closer.CloseAll(ctx)

	os.Exit(code)
}

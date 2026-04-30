package test

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/services/conflicts"
	"github.com/ruko1202/maintmode/internal/services/maint"
	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	conflictsStore "github.com/ruko1202/maintmode/internal/storages/conflicts"

	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db             *sqlx.DB
	maintStore     *maintenances.Store
	resourcesStore *resources.Store
	conflictsSrv   *conflicts.Service
	snapshotsStore *conflictsnapshots.Store
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	xlog.ReplaceGlobalLogger(xlog.NewZapAdapter(logger))

	conn := testdbconnutils.NewDB()
	closer.Add(conn.Close)
	db = conn

	maintStore = maintenances.NewStore(conn)
	resourcesStore = resources.NewStore(conn)
	conflictsSrv = conflicts.NewService(
		conflictsStore.NewStore(db),
		conflictsnapshots.NewStore(db),
	)
	snapshotsStore = conflictsnapshots.NewStore(db)

	code := m.Run()

	closer.CloseAll(ctx)

	os.Exit(code)
}

func initService(db *sqlx.DB) *maint.Service {
	return maint.NewService(
		dbtx.NewTxManager(db),
		maintStore,
		resourcesStore,
		conflictsSrv,
	)
}

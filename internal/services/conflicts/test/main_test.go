package test

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/services/conflicts"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	conflictsnapshots "github.com/ruko1202/maintmode/internal/storages/conflict_snapshots"
	conflictsStore "github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var (
	db         *sqlx.DB
	maintStore *maintenances.Store
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	xlog.ReplaceGlobal(logger)

	conn := testdbconnutils.NewDB()
	db = conn
	closer.Add(conn.Close)

	maintStore = maintenances.NewStore(db)
	code := m.Run()

	closer.CloseAll(ctx)

	os.Exit(code)
}

func initService(db *sqlx.DB) *conflicts.Service {
	return conflicts.NewService(
		conflictsStore.NewStore(db),
		conflictsnapshots.NewStore(db),
	)
}

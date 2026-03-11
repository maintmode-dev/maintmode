package maint

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
	"github.com/ruko1202/maintmode/internal/utils/dbtx"

	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	xlog.ReplaceGlobalLogger(xlog.NewZapAdapter(logger))

	conn := testdbconnutils.NewDB()
	closer.Add(conn.Close)
	db = conn

	code := m.Run()

	closer.CloseAll(ctx)

	os.Exit(code)
}

func initService(db *sqlx.DB) *Service {
	return NewService(
		dbtx.NewTxManager(db),
		maintenances.NewStore(db),
		resources.NewStore(db),
		nil,
	)
}

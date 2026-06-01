package infra

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/lifecycle"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db *sqlx.DB
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAuthAppConfig())
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

func initImpl(t *testing.T) *Implementation {
	t.Helper()

	return New(db, lifecycle.NewDrainer())
}

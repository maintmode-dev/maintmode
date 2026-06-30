package users

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

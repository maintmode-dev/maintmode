package notifytargets

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/ruko1202/maintmode/internal/config"
	notifychannelstore "github.com/ruko1202/maintmode/internal/storages/notifychannel"
	notifytargetsstore "github.com/ruko1202/maintmode/internal/storages/notifytargets"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db  *sqlx.DB
	svc *Service
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	svc = NewService(
		dbtx.NewTxManager(db),
		notifychannelstore.NewStore(db),
		notifytargetsstore.NewStore(db),
	)

	os.Exit(m.Run())
}

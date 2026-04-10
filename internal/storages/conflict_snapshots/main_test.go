package conflictsnapshots

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var db *sqlx.DB
var (
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

	maintsStore = maintenances.NewStore(db)
	resourcesStore = resources.NewStore(db)

	code := m.Run()

	closer.CloseAll(ctx)

	os.Exit(code)
}

func makeResource(ctx context.Context, t *testing.T) *entity.ResourceDetails {
	t.Helper()

	resource, err := resourcesStore.Create(ctx, &entity.ResourceDetails{
		Name:        "Resource" + xuuid.NewString(),
		Description: "Description 1",
	})
	require.NoError(t, err)

	return resource
}

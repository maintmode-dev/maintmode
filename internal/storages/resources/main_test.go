package resources

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"

	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	xlog.ReplaceGlobal(logger)

	conn := testdbconnutils.NewDB()
	db = conn
	closer.Add(conn.Close)

	code := m.Run()

	closer.CloseAll(ctx)

	os.Exit(code)
}

func makeResource(ctx context.Context, t *testing.T, store *Store) *entity.ResourceDetails {
	t.Helper()

	resource := &entity.ResourceDetails{
		ID:          xuuid.New(),
		Name:        "Name" + t.Name() + xuuid.NewString(),
		Description: "Description" + t.Name(),
		ExternalID:  lo.ToPtr(xuuid.NewString()),
		CreatedAt:   xtime.UTCNow(),
	}

	err := store.Create(ctx, resource)
	require.NoError(t, err)
	require.NotNil(t, resource)

	return resource
}

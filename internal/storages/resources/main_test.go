package resources

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/utils/closer"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	code := m.Run()

	os.Exit(code)
}

func makeResource(ctx context.Context, t *testing.T, store *Store) *entity.ResourceDetails {
	t.Helper()

	resource, err := store.Create(ctx, &entity.ResourceDetails{
		Name:        "Name" + t.Name() + xuuid.NewString(),
		Description: "Description" + t.Name(),
		ExternalID:  lo.ToPtr(xuuid.NewString()),
		Status:      entity.ResourceStatusActive,
	})
	require.NoError(t, err)
	require.NotNil(t, resource)

	return resource
}

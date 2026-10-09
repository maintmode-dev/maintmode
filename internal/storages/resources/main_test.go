package resources

import (
	"context"
	"os"
	"testing"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/table"
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

	return makeNamedResource(ctx, t, store, "Name"+t.Name()+xuuid.NewString())
}

// makeNamedResource creates an active resource with the given exact name. Tests
// that need a private, race-free slice of the shared list use a unique name
// token so List(Name: token) returns only their own rows.
func makeNamedResource(ctx context.Context, t *testing.T, store *Store, name string) *entity.ResourceDetails {
	t.Helper()

	resource, err := store.Create(ctx, &entity.ResourceDetails{
		Name:        name,
		Description: "Description" + t.Name(),
		ExternalID:  lo.ToPtr(xuuid.NewString()),
		Status:      entity.ResourceStatusActive,
	})
	require.NoError(t, err)
	require.NotNil(t, resource)
	deleteResourceOnCleanup(t, resource.ID)

	return resource
}

// deleteResourceOnCleanup removes a resource the test created once the test
// ends. These tests write to the shared dev database, where a leftover row is
// not inert: it shows up in the dev stand's resource list and in anything
// captured from it. Nothing in this package references a resource (no
// maintenance links one), so a plain delete by id is enough.
func deleteResourceOnCleanup(t *testing.T, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		// Not t.Context(): it is already canceled when cleanups run.
		_, err := table.Resources.DELETE().
			WHERE(table.Resources.ID.EQ(postgres.UUID(id))).
			ExecContext(context.Background(), db)
		require.NoError(t, err, "clean up resource %s", id)
	})
}

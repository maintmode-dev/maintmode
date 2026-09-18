package authsettings_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	authsettingssvc "github.com/ruko1202/maintmode/internal/services/authsettings"
	authsettingsstore "github.com/ruko1202/maintmode/internal/storages/authsettings"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
	publishermock "github.com/ruko1202/maintmode/test/utils/mocks/publisher"
)

var db *sqlx.DB

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	code := m.Run()
	os.Exit(code)
}

// initService builds a service on the shared database with a per-test audit
// spy.
//
// The suite mutates the two seeded rows and restores them, because the table is
// a closed set: a test cannot insert its own row without inventing a method
// name nothing reads. That makes these tests non-parallel with each other by
// construction, which is why none of them call t.Parallel().
func initService(t *testing.T) (*authsettingssvc.Service, *publishermock.Spy) {
	t.Helper()

	spy := publishermock.New(t)
	svc := authsettingssvc.NewService(
		dbtx.NewTxManager(db),
		authsettingsstore.NewStore(db),
		spy,
	)

	restoreAll(t)

	return svc, spy
}

// restoreAll snapshots every flag and puts them all back afterwards.
func restoreAll(t *testing.T) {
	t.Helper()

	store := authsettingsstore.NewStore(db)

	before, err := store.List(context.Background())
	require.NoError(t, err)

	t.Cleanup(func() {
		for _, s := range before {
			_, restoreErr := store.Update(context.Background(), s)
			require.NoError(t, restoreErr)
		}
	})
}

// enableAll turns every built-in on, as an explicit PRECONDITION.
//
// The suite used to inherit "both methods are on" from the migration's seed,
// which made a seed change land as failures in tests that are not about the
// seed at all -- and would have made them pass again for the wrong reason if
// the seed changed back. A test that needs a starting state now says so.
//
// Safe to mutate because initService has already registered the restore.
func enableAll(t *testing.T, svc *authsettingssvc.Service) {
	t.Helper()

	for _, method := range entity.AllAuthMethodNames() {
		_, err := svc.SetEnabled(context.Background(), &entity.SetAuthMethodEnabledCmd{
			Method:  method,
			Enabled: true,
			Actor:   testActor(),
		})
		require.NoError(t, err)
	}
}

func testActor() *entity.User {
	return &entity.User{ID: uuid.New(), Email: "admin@example.com"}
}

func enabledOf(t *testing.T, svc *authsettingssvc.Service, method entity.AuthMethodName) bool {
	t.Helper()

	got, err := svc.Enabled(context.Background(), method)
	require.NoError(t, err)

	return got
}

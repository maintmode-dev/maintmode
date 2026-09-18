package authsettings

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db    *sqlx.DB
	store *Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	store = NewStore(db)

	code := m.Run()
	os.Exit(code)
}

// restoreEnabled puts a method back the way the test found it.
//
// Unlike the other stores' suites, this one cannot uniquify its data: the table
// is a CLOSED set of two seeded rows, so a test cannot insert its own row
// without inventing a method name nothing reads. It mutates the shared rows and
// restores them instead -- which also means these tests must not run in
// parallel with each other, and they do not.
func restoreEnabled(t *testing.T, method entity.AuthMethodName, enabled bool) {
	t.Helper()

	t.Cleanup(func() {
		settings, err := store.List(context.Background())
		require.NoError(t, err)

		for _, s := range settings {
			if s.Method != method {
				continue
			}

			s.Enabled = enabled
			s.UpdatedByUserID = nil
			_, err = store.Update(context.Background(), s)
			require.NoError(t, err)
		}
	})
}

// get returns one setting by method, failing the test if it is absent.
func get(t *testing.T, method entity.AuthMethodName) *entity.AuthMethodSetting {
	t.Helper()

	settings, err := store.List(context.Background())
	require.NoError(t, err)

	for _, s := range settings {
		if s.Method == method {
			return s
		}
	}

	require.Failf(t, "method missing", "no row for %q", method)

	return nil
}

package dekrotator

import (
	"context"
	"encoding/hex"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
	"github.com/ruko1202/maintmode/internal/storages/datakey"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/dbtx"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db        *sqlx.DB
	store     *datakey.Store
	txManager *dbtx.TxManager
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	store = datakey.NewStore(db)
	txManager = dbtx.NewTxManager(db)

	code := m.Run()
	os.Exit(code)
}

// uniqueKEKURI returns a KEK URI that no other test, package, or previous run
// shares. Rotation scans the WHOLE data_keys table, so instead of wiping shared
// tables (which would race concurrent suites on the shared dev DB), each test
// isolates by key identity: every foreign row — including this test's own
// earlier runs — sits under a KEK URI this test's keyring does not know, so
// rotation skips it, never re-wraps it, and never counts it in `rewrapped`.
func uniqueKEKURI(name string) string {
	return "local-kms://" + name + "-" + xuuid.NewString()
}

// seedDEK generates a fresh DEK, wraps it with kr, and stores it. It returns the
// stored row and the plaintext DEK so tests can assert the DEK survives rotation.
func seedDEK(ctx context.Context, t *testing.T, kr *secrets.Keyring, label string) (row *entity.DataKey, dek []byte) {
	t.Helper()

	dek, err := secrets.GenerateDEK()
	require.NoError(t, err)

	wrapped, kekID, err := kr.WrapDEK(dek)
	require.NoError(t, err)

	uniqueLabel := label + "-" + xuuid.NewString()
	stored, err := store.Create(ctx, &entity.DataKey{
		KEKID:        kekID,
		EncryptedDEK: wrapped,
		Label:        &uniqueLabel,
	})
	require.NoError(t, err)
	return stored, dek
}

// findByID loads a data_keys row by id, failing the test on any error.
func findByID(ctx context.Context, t *testing.T, id uuid.UUID) *entity.DataKey {
	t.Helper()
	dk, err := store.GetByID(ctx, id)
	require.NoError(t, err)
	return dk
}

// keyringWith builds a keyring over the given hex KEKs with active set to the
// first id. hexKEK generates deterministic, valid, distinct 32-byte keys.
func keyringWith(t *testing.T, active string, ids ...string) *secrets.Keyring {
	t.Helper()

	keys := make(map[string]string, len(ids))
	for i, id := range ids {
		keys[id] = hexKEK(byte(i + 1))
	}
	kr, err := secrets.NewLocalKeyring(active, keys)
	require.NoError(t, err)
	return kr
}

func hexKEK(seed byte) string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = seed + byte(i)
	}
	return hex.EncodeToString(key)
}

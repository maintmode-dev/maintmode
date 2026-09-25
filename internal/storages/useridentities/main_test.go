package useridentities

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
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

// seedUser inserts a user directly: the users store lives in another package,
// and what these tests need is a parent row for the user_id FK.
func seedUser(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := db.QueryRowxContext(ctx,
		`INSERT INTO users (email, name, roles) VALUES ($1, $2, '{guest}') RETURNING id`,
		xuuid.NewString()+"@identities-test.com", "Identity Test User").Scan(&id)
	require.NoError(t, err)

	return id
}

// seedProvider inserts a login row in the registry and returns its id, so an
// identity has a parent to reference. The chain is three deep -- data_keys →
// integration_settings → user_identities -- because integration_settings.dek_id
// is itself a foreign key.
//
// The name is unique per call: the suite runs -count 2 against a shared
// database and integration_settings is unique on (kind, name).
func seedProvider(ctx context.Context, t *testing.T) uuid.UUID {
	t.Helper()

	return seedNamedProvider(ctx, t, "provider-"+xuuid.NewString())
}

// seedNamedProvider is seedProvider with the name chosen by the caller, for a
// test that needs the name order to differ from the id order.
func seedNamedProvider(ctx context.Context, t *testing.T, name string) uuid.UUID {
	t.Helper()

	var dekID uuid.UUID
	err := db.QueryRowxContext(ctx,
		`INSERT INTO data_keys (kek_id, encrypted_dek) VALUES ($1, $2) RETURNING id`,
		"identities-test-kek", []byte("wrapped")).Scan(&dekID)
	require.NoError(t, err)

	var id uuid.UUID
	err = db.QueryRowxContext(ctx,
		`INSERT INTO integration_settings (kind, name, enabled, config, secrets, dek_id)
		 VALUES ('login', $1, true, '{}'::jsonb, '{}'::jsonb, $2) RETURNING id`,
		name, dekID).Scan(&id)
	require.NoError(t, err)

	return id
}

// identity builds an unstored identity for one user, with a unique subject.
func identity(userID uuid.UUID, ref entity.SignInMethodRef) *entity.UserIdentity {
	row := &entity.UserIdentity{
		UserID:  userID,
		Subject: xuuid.NewString(),
		Email:   xuuid.NewString() + "@identities-test.com",
	}
	ref.Apply(row)

	return row
}

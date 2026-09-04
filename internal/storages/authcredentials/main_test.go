package authcredentials

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/closer"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db         *sqlx.DB
	store      *Store
	usersStore *users.Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	store = NewStore(db)
	usersStore = users.NewStore(db)

	code := m.Run()
	os.Exit(code)
}

// makeUser inserts a user to own the credentials under test. The suite runs
// with -count 2 against a shared database, so the email is randomised rather
// than derived from the test name: a reused address would collide on the second
// pass and surface as a conflict failure that looks exactly like a genuine
// conflict-detection bug.
func makeUser(ctx context.Context, t *testing.T) *entity.User {
	t.Helper()

	u, err := usersStore.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "auth credentials test user",
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)
	require.NotNil(t, u)

	return u
}

// makeOTP inserts a live one-time code for the user.
func makeOTP(ctx context.Context, t *testing.T, userID uuid.UUID) *entity.AuthCredential {
	t.Helper()

	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	nonce := uuid.NewString()

	cred, err := store.Create(ctx, &entity.AuthCredential{
		UserID:       userID,
		Kind:         entity.AuthCredentialKindOTP,
		SecretHash:   uuid.NewString(),
		ExpiresAt:    &expiresAt,
		SessionNonce: &nonce,
	})
	require.NoError(t, err)
	require.NotNil(t, cred)

	return cred
}

// makePassword inserts a password credential for the user.
func makePassword(ctx context.Context, t *testing.T, userID uuid.UUID) *entity.AuthCredential {
	t.Helper()

	cred, err := store.Create(ctx, &entity.AuthCredential{
		UserID:     userID,
		Kind:       entity.AuthCredentialKindPassword,
		SecretHash: "$argon2id$v=19$m=65536,t=3,p=4$" + uuid.NewString(),
	})
	require.NoError(t, err)
	require.NotNil(t, cred)

	return cred
}

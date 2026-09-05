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
// with -count 2 against a shared database, so the email is randomized rather
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

// makeOTPExpiringAt inserts a one-time code with a caller-chosen expiry, so a
// test can place a row on either side of a prune cutoff. Create inserts
// expires_at straight from the entity, so no back-dating UPDATE is needed here —
// unlike user_invitations, whose created_at is stamped server-side.
func makeOTPExpiringAt(
	ctx context.Context,
	t *testing.T,
	userID uuid.UUID,
	expiresAt time.Time,
) *entity.AuthCredential {
	t.Helper()

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

// makePasswordExpiringAt inserts a password credential that carries an expiry.
//
// Nothing in the schema forbids one — no CHECK ties kind='password' to a NULL
// expires_at — and that is exactly why this fixture exists: it is the only row
// shape that proves the prune's kind guard does real work. A password row with a
// NULL expiry cannot prove it, because NULL < cutoff is never true and such a row
// survives whether or not the guard is there.
func makePasswordExpiringAt(
	ctx context.Context,
	t *testing.T,
	userID uuid.UUID,
	expiresAt time.Time,
) *entity.AuthCredential {
	t.Helper()

	cred, err := store.Create(ctx, &entity.AuthCredential{
		UserID:     userID,
		Kind:       entity.AuthCredentialKindPassword,
		SecretHash: "$argon2id$v=19$m=65536,t=3,p=4$" + uuid.NewString(),
		ExpiresAt:  &expiresAt,
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

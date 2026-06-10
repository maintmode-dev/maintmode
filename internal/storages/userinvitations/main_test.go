package userinvitations

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
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"
)

var (
	db         *sqlx.DB
	store      *Store
	usersStore *users.Store
)

// userinvitations lives in the auth DB (users + user_invitations), so the tests
// load the auth config like the users store tests do.
func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAuthAppConfig())
	closer.Add(db.Close)

	store = NewStore(db)
	usersStore = users.NewStore(db)

	code := m.Run()
	os.Exit(code)
}

// makeInviter inserts a unique auth user to act as the invitation's inviter, so
// the List INNER JOIN on users has a row to resolve the inviter profile from.
func makeInviter(ctx context.Context, t *testing.T, name string) *entity.User {
	t.Helper()

	inviter, err := usersStore.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  name,
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)
	require.NotNil(t, inviter)

	return inviter
}

// makeInvitation inserts an invitation for the given inviter and email. The
// token hash is random so the token_hash unique index never collides, and
// expiresAt lets callers place the row on either side of the pending/expired
// boundary. sentAt is set to now.
func makeInvitation(
	ctx context.Context,
	t *testing.T,
	inviterID uuid.UUID,
	email string,
	status entity.InvitationStatus,
	expiresAt time.Time,
) *entity.Invitation {
	t.Helper()
	return makeInvitationAt(ctx, t, inviterID, email, status, expiresAt, xtime.UTCNow())
}

// makeInvitationAt is makeInvitation with an explicit sent_at, for tests that
// assert on the sent_at DESC ordering.
func makeInvitationAt(
	ctx context.Context,
	t *testing.T,
	inviterID uuid.UUID,
	email string,
	status entity.InvitationStatus,
	expiresAt time.Time,
	sentAt time.Time,
) *entity.Invitation {
	t.Helper()

	inv, err := store.Create(ctx, &entity.Invitation{
		Email:       email,
		Roles:       entity.DefaultRoles,
		TokenHash:   uuid.NewString(),
		Status:      status,
		InvitedByID: inviterID,
		ExpiresAt:   expiresAt,
		SentAt:      sentAt,
	})
	require.NoError(t, err)
	require.NotNil(t, inv)

	return inv
}

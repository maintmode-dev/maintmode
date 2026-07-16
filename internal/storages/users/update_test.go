package users

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func makeTimezoneUser(ctx context.Context, t *testing.T, store *Store) *entity.User {
	t.Helper()
	created, err := store.Create(ctx, &entity.User{
		Email: uuid.NewString() + "@email.com",
		Name:  "Name-" + uuid.NewString(),
		Roles: entity.DefaultRoles,
	})
	require.NoError(t, err)
	require.Nil(t, created.Timezone, "new user has no timezone")
	return created
}

// TestUpdate_SetTimezone verifies the timezone column round-trips through Update
// and GetByID. The read-back is asserted against a concrete value on purpose: a
// mis-tagged scan column reads silently empty (see the qrm alias-vs-db tag
// pitfall), so equality is what actually proves the column is wired.
func TestUpdate_SetTimezone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)
	user := makeTimezoneUser(ctx, t, store)

	user.Timezone = lo.ToPtr("Asia/Nicosia")
	require.NoError(t, store.Update(ctx, user))

	got, err := store.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Timezone)
	require.Equal(t, "Asia/Nicosia", *got.Timezone)
}

// TestUpdate_ResetTimezone verifies clearing the preference persists as NULL.
func TestUpdate_ResetTimezone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)
	user := makeTimezoneUser(ctx, t, store)

	user.Timezone = lo.ToPtr("Europe/Berlin")
	require.NoError(t, store.Update(ctx, user))

	user.Timezone = nil
	require.NoError(t, store.Update(ctx, user))

	got, err := store.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Nil(t, got.Timezone)
}

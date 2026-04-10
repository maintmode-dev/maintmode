package users

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	user := &entity.User{
		Email:           uuid.NewString() + "@email.com",
		Name:            "Name" + t.Name(),
		OAuthProviderID: "OAuthProviderID" + uuid.NewString() + t.Name(),
		Roles:           entity.DefaultRoles,
	}

	created, err := store.Create(ctx, user)
	require.NoError(t, err)
	require.NotNil(t, created)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		t.Run("GetByID", func(t *testing.T) {
			t.Parallel()

			dbUser, err := store.GetByID(ctx, created.ID)
			require.NoError(t, err)
			require.NotNil(t, dbUser)
			require.Equal(t, created, dbUser)
		})

		t.Run("GetByOAuthProviderID", func(t *testing.T) {
			t.Parallel()

			dbUser, err := store.GetByOAuthProviderID(ctx, created.OAuthProviderID)
			require.NoError(t, err)
			require.NotNil(t, dbUser)
			require.Equal(t, created, dbUser)
		})
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		t.Run(apperr.ErrUserNotFound.Error(), func(t *testing.T) {
			t.Run("GetByID", func(t *testing.T) {
				t.Parallel()

				dbUser, err := store.GetByID(ctx, uuid.New())
				require.Nil(t, dbUser)
				require.EqualError(t, err, apperr.ErrUserNotFound.Error())
			})

			t.Run("GetByOAuthProviderID", func(t *testing.T) {
				t.Parallel()

				dbUser, err := store.GetByOAuthProviderID(ctx, "some google id")
				require.Nil(t, dbUser)
				require.EqualError(t, err, apperr.ErrUserNotFound.Error())
			})
		})
	})
}

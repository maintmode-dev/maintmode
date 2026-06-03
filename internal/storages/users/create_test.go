package users

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		user := &entity.User{
			Email: uuid.NewString() + "@email.com",
			Name:  "Name" + t.Name(),
			Roles: entity.DefaultRoles,
		}

		created, err := store.Create(ctx, user)
		require.NoError(t, err)
		require.NotNil(t, created)

		user.ID = created.ID
		user.CreatedAt = created.CreatedAt
		require.Equal(t, user, created)

		dbUser, err := store.GetByID(ctx, created.ID)
		require.NoError(t, err)
		require.NotNil(t, dbUser)
		require.Equal(t, created, dbUser)
	})
}

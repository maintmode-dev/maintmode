package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestAssignRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		user := makeUser(ctx, t, srv)
		err := srv.AssignRole(ctx, &entity.AssignRoleCmd{
			Actor:  &entity.User{},
			Role:   entity.RoleEditor,
			UserID: user.ID,
		})
		require.NoError(t, err)

		roles, err := srv.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, len(user.Roles)+1, len(roles))
		require.Equal(t, append(user.Roles, entity.RoleEditor), roles)
	})

	t.Run("already assigned", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		user := makeUser(ctx, t, srv, entity.RoleEditor)

		err := srv.AssignRole(ctx, &entity.AssignRoleCmd{
			Actor:  &entity.User{},
			Role:   entity.RoleEditor,
			UserID: user.ID,
		})
		require.NoError(t, err)

		roles, err := srv.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, len(user.Roles), len(roles))
		require.Equal(t, user.Roles, roles)
	})

	t.Run("invalid role", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		user := makeUser(ctx, t, srv)

		err := srv.AssignRole(ctx, &entity.AssignRoleCmd{
			Actor:  &entity.User{},
			Role:   "superuser",
			UserID: user.ID,
		})
		require.ErrorIs(t, err, apperr.ErrInvalidRole)
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		err := srv.AssignRole(ctx, &entity.AssignRoleCmd{
			Actor:  &entity.User{},
			Role:   entity.RoleAdmin,
			UserID: uuid.New(),
		})
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
	})
}

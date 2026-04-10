package user

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestRemoveRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		user := makeUser(ctx, t, srv, entity.RoleEditor, entity.RoleAdmin)

		err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  &entity.User{},
			UserID: user.ID,
			Role:   entity.RoleEditor,
		})
		require.NoError(t, err)

		roles, err := srv.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, len(user.Roles)-1, len(roles))
		require.Equal(t, slices.DeleteFunc(user.Roles, func(role entity.Role) bool {
			return role == entity.RoleEditor
		}), roles)
	})

	t.Run("not present", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		user := makeUser(ctx, t, srv, entity.RoleEditor)

		err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  &entity.User{},
			UserID: user.ID,
			Role:   entity.RoleAdmin,
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

		err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  &entity.User{},
			UserID: user.ID,
			Role:   "superuser",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidRole)
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  &entity.User{},
			UserID: uuid.New(),
			Role:   entity.RoleAdmin,
		})
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
	})
}

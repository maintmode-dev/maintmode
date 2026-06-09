package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestReplaceRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		user := makeUser(ctx, t, srv, entity.RoleReviewer, entity.RoleEditor, entity.RoleAdmin)

		newRoles := []entity.Role{entity.RoleAdmin, entity.RoleEditor}
		rolesInput := make([]entity.Role, 0, 4)
		rolesInput = append(rolesInput, newRoles...)
		rolesInput = append(rolesInput, entity.RoleEditor, "superuser")

		err := srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{},
			UserID: user.ID,
			Roles:  rolesInput,
		})
		require.NoError(t, err)

		roles, err := srv.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, len(newRoles), len(roles))
		require.Equal(t, newRoles, roles)
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		err := srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{},
			UserID: uuid.New(),
			Roles:  []entity.Role{entity.RoleAdmin},
		})
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
	})

	t.Run("self replace is rejected", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)

		user := makeUser(ctx, t, srv, entity.RoleEditor, entity.RoleAdmin)

		err := srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{ID: user.ID},
			UserID: user.ID,
			Roles:  []entity.Role{entity.RoleEditor},
		})
		require.ErrorIs(t, err, apperr.ErrSelfRevoke)

		// The role set must be untouched after a rejected self-replace.
		roles, err := srv.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		require.ElementsMatch(t, user.Roles, roles)
	})
}

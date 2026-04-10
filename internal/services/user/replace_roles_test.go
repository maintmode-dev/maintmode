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
		err := srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{},
			UserID: user.ID,
			Roles:  append(newRoles, entity.RoleEditor, "superuser"),
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
}

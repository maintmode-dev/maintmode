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

	t.Run("stripping admin from the last active admin is rejected", func(t *testing.T) {
		t.Parallel()

		srv := initServiceWithAdminCount(t, 1)
		admin := makeUser(ctx, t, srv, entity.RoleAdmin)

		err := srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{},
			UserID: admin.ID,
			Roles:  []entity.Role{entity.RoleEditor},
		})
		require.ErrorIs(t, err, apperr.ErrLastAdmin)

		// The role set must be untouched after the rejected replace.
		roles, err := srv.GetRoles(ctx, admin.ID)
		require.NoError(t, err)
		require.ElementsMatch(t, admin.Roles, roles)
	})

	t.Run("stripping admin from a non-last admin is allowed", func(t *testing.T) {
		t.Parallel()

		srv := initServiceWithAdminCount(t, 2)
		admin := makeUser(ctx, t, srv, entity.RoleAdmin)

		err := srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{},
			UserID: admin.ID,
			Roles:  []entity.Role{entity.RoleEditor},
		})
		require.NoError(t, err)

		roles, err := srv.GetRoles(ctx, admin.ID)
		require.NoError(t, err)
		require.NotContains(t, roles, entity.RoleAdmin)
	})

	t.Run("replace keeping admin skips the guard", func(t *testing.T) {
		t.Parallel()

		// count=1: if the guard ran, it would reject — a replace that keeps the
		// admin role cannot shrink the admin count, so it must pass.
		srv := initServiceWithAdminCount(t, 1)
		admin := makeUser(ctx, t, srv, entity.RoleAdmin)

		err := srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{},
			UserID: admin.ID,
			Roles:  []entity.Role{entity.RoleAdmin, entity.RoleEditor},
		})
		require.NoError(t, err)

		roles, err := srv.GetRoles(ctx, admin.ID)
		require.NoError(t, err)
		require.ElementsMatch(t, []entity.Role{entity.RoleAdmin, entity.RoleEditor}, roles)
	})

	t.Run("replace on a non-admin skips the guard", func(t *testing.T) {
		t.Parallel()

		// The target never held admin, so this replace cannot shrink the admin
		// count: it must pass even at count=1. (The guard is a no-op on a
		// non-admin regardless, so this pins the invariant, not the exact
		// stripping-admin condition.)
		srv := initServiceWithAdminCount(t, 1)
		user := makeUser(ctx, t, srv, entity.RoleEditor)

		err := srv.ReplaceRoles(ctx, &entity.ReplaceRolesCmd{
			Actor:  &entity.User{},
			UserID: user.ID,
			Roles:  []entity.Role{entity.RoleReviewer},
		})
		require.NoError(t, err)

		roles, err := srv.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		require.ElementsMatch(t, []entity.Role{entity.RoleReviewer}, roles)
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

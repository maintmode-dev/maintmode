package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestAssignRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initService(t)
	actor := makeUser(ctx, t, srv)

	t.Run("ok single", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		updated, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleEditor},
			UserID: user.ID,
		})
		require.NoError(t, err)
		require.Contains(t, updated.Roles, entity.RoleEditor)

		roles, err := srv.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, append(user.Roles, entity.RoleEditor), roles)
	})

	t.Run("ok multiple in one call", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)
		updated, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleEditor, entity.RoleReviewer},
			UserID: user.ID,
		})
		require.NoError(t, err)
		require.Contains(t, updated.Roles, entity.RoleEditor)
		require.Contains(t, updated.Roles, entity.RoleReviewer)
	})

	t.Run("already assigned is a no-op and returns the user", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv, entity.RoleEditor)

		updated, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleEditor},
			UserID: user.ID,
		})
		require.NoError(t, err)
		require.NotNil(t, updated)
		require.Equal(t, user.Roles, updated.Roles)
	})

	t.Run("union keeps existing roles", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv, entity.RoleEditor)

		updated, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleReviewer},
			UserID: user.ID,
		})
		require.NoError(t, err)
		require.Contains(t, updated.Roles, entity.RoleEditor)
		require.Contains(t, updated.Roles, entity.RoleReviewer)
	})

	t.Run("invalid role", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv)

		_, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{"superuser"},
			UserID: user.ID,
		})
		require.ErrorIs(t, err, apperr.ErrInvalidRole)
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()

		_, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleAdmin},
			UserID: uuid.New(),
		})
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
	})
}

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

	srv := initService(t)
	actor := makeUser(ctx, t, srv, entity.RoleEditor, entity.RoleAdmin)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		user := makeUser(ctx, t, srv, entity.RoleEditor, entity.RoleAdmin)

		err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  actor,
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

		user := makeUser(ctx, t, srv, entity.RoleEditor)

		err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  actor,
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

		user := makeUser(ctx, t, srv)

		err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  actor,
			UserID: user.ID,
			Role:   "superuser",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidRole)
	})

	t.Run("user not found", func(t *testing.T) {
		t.Parallel()

		err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  actor,
			UserID: uuid.New(),
			Role:   entity.RoleAdmin,
		})
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
	})

	t.Run("revoking admin from a non-last admin is allowed", func(t *testing.T) {
		t.Parallel()

		// Ensure more than one active admin exists so the last-admin guard does
		// not trip (the shared DB already carries many admins).
		_ = makeUser(ctx, t, srv, entity.RoleAdmin)
		user := makeUser(ctx, t, srv, entity.RoleEditor, entity.RoleAdmin)

		err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
			Actor:  actor,
			UserID: user.ID,
			Role:   entity.RoleAdmin,
		})
		require.NoError(t, err)

		roles, err := srv.GetRoles(ctx, user.ID)
		require.NoError(t, err)
		require.NotContains(t, roles, entity.RoleAdmin)
	})
}

// TestRevokeRole_SelfRevokeGuard verifies an actor cannot revoke a role from
// themselves — the self-lockout vector. Mirrors the self-block guard.
func TestRevokeRole_SelfRevokeGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initService(t)
	user := makeUser(ctx, t, srv, entity.RoleEditor, entity.RoleAdmin)

	err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
		Actor:  &entity.User{ID: user.ID},
		UserID: user.ID,
		Role:   entity.RoleEditor,
	})
	require.ErrorIs(t, err, apperr.ErrSelfRevoke)

	// The role set must be untouched after a rejected self-revoke.
	roles, err := srv.GetRoles(ctx, user.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, user.Roles, roles)
}

// TestRevokeRole_LastAdminGuard verifies the last-admin lockout guard rejects
// revoking the admin role when that user is the only active admin. The active
// admin count is forced to 1 via initServiceWithAdminCount so the assertion is
// deterministic regardless of the shared dev DB's real admin population.
func TestRevokeRole_LastAdminGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv := initServiceWithAdminCount(t, 1)
	admin := makeUser(ctx, t, srv, entity.RoleAdmin)

	err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
		Actor:  &entity.User{},
		UserID: admin.ID,
		Role:   entity.RoleAdmin,
	})
	require.ErrorIs(t, err, apperr.ErrLastAdmin)
}

// TestRevokeRole_BlockedAdminSkipsLastAdminGuard verifies the guard's "active"
// qualifier: a blocked admin does not count towards admin availability, so
// stripping the admin role from them must pass even when the active-admin
// count is at the lockout threshold. If the guard checked IsAdmin instead of
// IsActiveAdmin, cleaning up a blocked admin's roles would be impossible once
// only one active admin remains.
func TestRevokeRole_BlockedAdminSkipsLastAdminGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Block the target while the forced count reports 2 active admins, so the
	// block itself passes the guard.
	blockSrv := initServiceWithAdminCount(t, 2)
	actor := makeUser(ctx, t, blockSrv, entity.RoleAdmin)
	target := makeUser(ctx, t, blockSrv, entity.RoleAdmin)
	require.NoError(t, blockSrv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID}))

	// count=1: if the guard ran for the blocked target, it would reject. A
	// blocked admin is not an active admin — the guard must early-return
	// (before taking the serialization lock) and let the revoke through.
	revokeSrv := initServiceWithAdminCount(t, 1)
	err := revokeSrv.RevokeRole(ctx, &entity.RevokeRoleCmd{
		Actor:  actor,
		UserID: target.ID,
		Role:   entity.RoleAdmin,
	})
	require.NoError(t, err)

	roles, err := revokeSrv.GetRoles(ctx, target.ID)
	require.NoError(t, err)
	require.NotContains(t, roles, entity.RoleAdmin)
}

// TestRevokeRole_NonAdminRoleSkipsLastAdminGuard verifies revoking a non-admin
// role is never serialized or rejected by the last-admin guard: it cannot
// shrink the admin count, so it must pass even when the target is the last
// active admin. Pins the "no new contention for ordinary role mutations"
// property of the guard's call sites.
func TestRevokeRole_NonAdminRoleSkipsLastAdminGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// count=1: if the guard ran for an editor revoke, it would reject because
	// the target is an active admin at the lockout threshold.
	srv := initServiceWithAdminCount(t, 1)
	actor := makeUser(ctx, t, srv, entity.RoleAdmin)
	target := makeUser(ctx, t, srv, entity.RoleAdmin, entity.RoleEditor)

	err := srv.RevokeRole(ctx, &entity.RevokeRoleCmd{
		Actor:  actor,
		UserID: target.ID,
		Role:   entity.RoleEditor,
	})
	require.NoError(t, err)

	roles, err := srv.GetRoles(ctx, target.ID)
	require.NoError(t, err)
	require.Contains(t, roles, entity.RoleAdmin)
	require.NotContains(t, roles, entity.RoleEditor)
}

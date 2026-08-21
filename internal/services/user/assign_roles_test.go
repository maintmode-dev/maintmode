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

func TestAssignRoles_SeatCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// setup uses a Noop-guard service so fixtures are never capped; the mutation
	// under test runs through a service with the controllable guard.
	setup := initService(t)
	actor := makeUser(ctx, t, setup)

	t.Run("guest to seat at full cap is rejected", func(t *testing.T) {
		t.Parallel()
		guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded}
		srv := initServiceWithSeatGuard(t, guard)
		u := makeUser(ctx, t, setup) // guest

		_, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleEditor},
			UserID: u.ID,
		})
		require.ErrorIs(t, err, apperr.ErrSeatsLimitExceeded)
		require.Equal(t, 1, guard.callCount(), "a guest→seat transition must consult the guard")
	})

	t.Run("guest to seat filling the last seat passes (off-by-one)", func(t *testing.T) {
		t.Parallel()
		guard := &fakeSeatGuard{} // guard reports room: the last seat is grantable
		srv := initServiceWithSeatGuard(t, guard)
		u := makeUser(ctx, t, setup)

		updated, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleReviewer},
			UserID: u.ID,
		})
		require.NoError(t, err)
		require.Contains(t, updated.Roles, entity.RoleReviewer)
		require.Equal(t, 1, guard.callCount())
	})

	t.Run("adding a second seat role to an already-seat user skips the guard", func(t *testing.T) {
		t.Parallel()
		guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded} // cap full, but no new seat
		srv := initServiceWithSeatGuard(t, guard)
		u := makeUser(ctx, t, setup, entity.RoleEditor) // already a seat

		updated, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleReviewer},
			UserID: u.ID,
		})
		require.NoError(t, err, "already-seat user consumes no new seat")
		require.Contains(t, updated.Roles, entity.RoleReviewer)
		require.Zero(t, guard.callCount(), "no non-seat→seat transition: guard must not fire")
	})

	t.Run("assigning a guest role never fires the guard", func(t *testing.T) {
		t.Parallel()
		guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded}
		srv := initServiceWithSeatGuard(t, guard)
		u := makeUser(ctx, t, setup) // guest with no roles yet

		_, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleGuest},
			UserID: u.ID,
		})
		require.NoError(t, err)
		require.Zero(t, guard.callCount())
	})

	t.Run("no-op re-assign at full cap skips the guard", func(t *testing.T) {
		t.Parallel()
		guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded}
		srv := initServiceWithSeatGuard(t, guard)
		u := makeUser(ctx, t, setup, entity.RoleEditor)

		// Re-assigning a role the user already holds is ErrNotChanged (rolls back),
		// so the guard must never fire — otherwise a full cap would 403 a no-op.
		updated, err := srv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  actor,
			Roles:  []entity.Role{entity.RoleEditor},
			UserID: u.ID,
		})
		require.NoError(t, err)
		require.Equal(t, u.Roles, updated.Roles)
		require.Zero(t, guard.callCount())
	})
}

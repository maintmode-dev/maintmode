package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestUnblockUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok clears blocked_at and preserves roles", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv, entity.RoleEditor, entity.RoleReviewer)

		require.NoError(t, srv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID}))
		require.NoError(t, srv.UnblockUser(ctx, &entity.UnblockUserCmd{Actor: actor, UserID: target.ID}))

		got, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.False(t, got.IsBlocked())
		require.Nil(t, got.BlockedAt)
		require.ElementsMatch(t, target.Roles, got.Roles)
	})

	t.Run("idempotent on active user", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv, entity.RoleEditor)

		// never blocked → unblock is a no-op returning nil
		require.NoError(t, srv.UnblockUser(ctx, &entity.UnblockUserCmd{Actor: actor, UserID: target.ID}))

		got, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.False(t, got.IsBlocked())
	})
}

func TestUnblockUser_SeatCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setup := initService(t) // Noop-guard: block/fixtures never capped
	actor := makeUser(ctx, t, setup, entity.RoleAdmin)

	t.Run("unblocking a seat-role user at full cap is rejected", func(t *testing.T) {
		t.Parallel()
		guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded}
		srv := initServiceWithSeatGuard(t, guard)
		target := makeUser(ctx, t, setup, entity.RoleEditor)
		require.NoError(t, setup.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID}))

		err := srv.UnblockUser(ctx, &entity.UnblockUserCmd{Actor: actor, UserID: target.ID})
		require.ErrorIs(t, err, apperr.ErrSeatsLimitExceeded)
		require.Equal(t, 1, guard.callCount(), "restoring a seat must consult the guard")

		// Rejected unblock leaves the user blocked.
		got, err := setup.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.True(t, got.IsBlocked())
	})

	t.Run("unblocking a guest user passes even at full cap", func(t *testing.T) {
		t.Parallel()
		guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded}
		srv := initServiceWithSeatGuard(t, guard)
		target := makeUser(ctx, t, setup, entity.RoleGuest)
		require.NoError(t, setup.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID}))

		err := srv.UnblockUser(ctx, &entity.UnblockUserCmd{Actor: actor, UserID: target.ID})
		require.NoError(t, err, "a guest restores no seat")
		require.Zero(t, guard.callCount())
	})

	t.Run("unblocking restoring the last seat passes (off-by-one)", func(t *testing.T) {
		t.Parallel()
		guard := &fakeSeatGuard{} // room for the seat being restored
		srv := initServiceWithSeatGuard(t, guard)
		target := makeUser(ctx, t, setup, entity.RoleReviewer)
		require.NoError(t, setup.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID}))

		err := srv.UnblockUser(ctx, &entity.UnblockUserCmd{Actor: actor, UserID: target.ID})
		require.NoError(t, err)
		require.Equal(t, 1, guard.callCount())
	})

	t.Run("unblocking an already-active user skips the guard", func(t *testing.T) {
		t.Parallel()
		guard := &fakeSeatGuard{err: apperr.ErrSeatsLimitExceeded}
		srv := initServiceWithSeatGuard(t, guard)
		target := makeUser(ctx, t, setup, entity.RoleEditor) // never blocked

		err := srv.UnblockUser(ctx, &entity.UnblockUserCmd{Actor: actor, UserID: target.ID})
		require.NoError(t, err, "no-op unblock (ErrNotChanged) must not fire the guard")
		require.Zero(t, guard.callCount())
	})
}

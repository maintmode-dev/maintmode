package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestBlockUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok blocks and revokes tokens", func(t *testing.T) {
		t.Parallel()

		srv, revoker := initServiceWithRevoker(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv, entity.RoleEditor)

		err := srv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID})
		require.NoError(t, err)

		got, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.True(t, got.IsBlocked())
		require.NotNil(t, got.BlockedAt)
		// roles preserved on block
		require.Equal(t, target.Roles, got.Roles)
		// target's sessions revoked
		require.Contains(t, revoker.revokedIDs(), target.ID)
	})

	t.Run("idempotent on already blocked", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv, entity.RoleEditor)

		require.NoError(t, srv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID}))
		// second block is a no-op, still 204-equivalent (nil)
		require.NoError(t, srv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID}))

		got, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.True(t, got.IsBlocked())
	})

	t.Run("self-block rejected", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)

		err := srv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: actor.ID})
		require.ErrorIs(t, err, apperr.ErrSelfBlock)

		got, err := srv.GetByID(ctx, actor.ID)
		require.NoError(t, err)
		require.False(t, got.IsBlocked())
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)

		err := srv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: uuid.New()})
		require.ErrorIs(t, err, apperr.ErrUserNotFound)
	})

	t.Run("blocking a non-last admin is allowed", func(t *testing.T) {
		t.Parallel()

		// Force more than one active admin so the last-admin guard does not trip,
		// independent of the shared DB's real admin population.
		srv := initServiceWithAdminCount(t, 2)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv, entity.RoleAdmin)

		err := srv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID})
		require.NoError(t, err)

		got, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.True(t, got.IsBlocked())
	})

	t.Run("blocking the last active admin is rejected", func(t *testing.T) {
		t.Parallel()

		// Force exactly one active admin so the guard trips deterministically.
		srv := initServiceWithAdminCount(t, 1)
		actor := makeUser(ctx, t, srv, entity.RoleAdmin)
		target := makeUser(ctx, t, srv, entity.RoleAdmin)

		err := srv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: target.ID})
		require.ErrorIs(t, err, apperr.ErrLastAdmin)

		got, err := srv.GetByID(ctx, target.ID)
		require.NoError(t, err)
		require.False(t, got.IsBlocked())
	})
}

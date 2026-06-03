package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

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

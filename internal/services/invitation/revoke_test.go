package invitation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestRevoke(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		err := svc.Revoke(ctx, &entity.RevokeInvitationCmd{Actor: makeAdmin(ctx, t, svc), ID: newUUID()})
		require.ErrorIs(t, err, apperr.ErrInvitationNotFound)
	})

	t.Run("idempotent on already revoked", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		inv := mustCreate(ctx, t, svc, uniqueEmail(t))

		require.NoError(t, svc.Revoke(ctx, &entity.RevokeInvitationCmd{Actor: makeAdmin(ctx, t, svc), ID: inv.ID}))
		require.NoError(t, svc.Revoke(ctx, &entity.RevokeInvitationCmd{Actor: makeAdmin(ctx, t, svc), ID: inv.ID}))
	})
}

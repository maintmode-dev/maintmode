package invitation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestResend(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("rotates token for pending", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		inv := mustCreate(ctx, t, svc, uniqueEmail(t))
		oldHash := inv.TokenHash

		require.NoError(t, svc.Resend(ctx, &entity.ResendInvitationCmd{Actor: makeAdmin(ctx, t, svc), ID: inv.ID}))

		got, err := svc.store.GetByID(ctx, inv.ID)
		require.NoError(t, err)
		require.NotEqual(t, oldHash, got.TokenHash)
		require.Equal(t, entity.InvitationStatusPending, got.Status)
	})

	t.Run("revoked cannot be resent", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		inv := mustCreate(ctx, t, svc, uniqueEmail(t))
		require.NoError(t, svc.Revoke(ctx, &entity.RevokeInvitationCmd{Actor: makeAdmin(ctx, t, svc), ID: inv.ID}))

		err := svc.Resend(ctx, &entity.ResendInvitationCmd{Actor: makeAdmin(ctx, t, svc), ID: inv.ID})
		require.ErrorIs(t, err, apperr.ErrInvitationNotPending)
	})
}

package invitation

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("fresh invitation shows under pending, not expired", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		emailAddr := uniqueEmail(t)
		inv := mustCreate(ctx, t, svc, emailAddr)

		// A freshly created invite is live, so it lists under pending and not
		// under expired.
		pending, err := svc.List(ctx, &entity.ListInvitationsCmd{Status: entity.InvitationStatusPending})
		require.NoError(t, err)
		require.True(t, containsInvitation(pending, inv.ID), "fresh invite must list as pending")

		expired, err := svc.List(ctx, &entity.ListInvitationsCmd{Status: entity.InvitationStatusExpired})
		require.NoError(t, err)
		require.False(t, containsInvitation(expired, inv.ID), "fresh invite must not list as expired")
	})

	t.Run("revoked invitation shows under revoked", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)
		inv := mustCreate(ctx, t, svc, uniqueEmail(t))
		require.NoError(t, svc.Revoke(ctx, &entity.RevokeInvitationCmd{Actor: makeAdmin(ctx, t, svc), ID: inv.ID}))

		revoked, err := svc.List(ctx, &entity.ListInvitationsCmd{Status: entity.InvitationStatusRevoked})
		require.NoError(t, err)
		require.True(t, containsInvitation(revoked, inv.ID))
	})
}

func containsInvitation(items []*entity.InvitationListItem, id uuid.UUID) bool {
	for _, it := range items {
		if it.Invitation.ID == id {
			return true
		}
	}
	return false
}

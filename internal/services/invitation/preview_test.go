package invitation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestPreview(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("unknown token is invalid (not an error)", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)

		status, err := svc.Preview(ctx, "no-such-token")
		require.NoError(t, err)
		require.Equal(t, entity.InvitationPreviewStatusInvalid, status)
	})

	t.Run("empty token is invalid (not an error)", func(t *testing.T) {
		t.Parallel()
		svc, _ := initService(t)

		status, err := svc.Preview(ctx, "")
		require.NoError(t, err)
		require.Equal(t, entity.InvitationPreviewStatusInvalid, status)
	})

	t.Run("revoked invitation previews as revoked", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initService(t)
		emailAddr := uniqueEmail(t)

		inv := mustCreate(ctx, t, svc, emailAddr)
		require.NoError(t, svc.Revoke(ctx, &entity.RevokeInvitationCmd{Actor: makeAdmin(ctx, t, svc), ID: inv.ID}))

		raw := rawTokenFromLink(t, mocks.sentEmail.body)
		status, err := svc.Preview(ctx, raw)
		require.NoError(t, err)
		require.Equal(t, entity.InvitationPreviewStatusRevoked, status)
	})

	t.Run("valid pending previews as valid", func(t *testing.T) {
		t.Parallel()
		svc, mocks := initService(t)

		mustCreate(ctx, t, svc, uniqueEmail(t))
		raw := rawTokenFromLink(t, mocks.sentEmail.body)

		status, err := svc.Preview(ctx, raw)
		require.NoError(t, err)
		require.Equal(t, entity.InvitationPreviewStatusValid, status)
	})
}

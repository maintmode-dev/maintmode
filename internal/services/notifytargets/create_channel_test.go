package notifytargets

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// The transport guard is tested HERE, at the point that enforces it, and not
// only as a predicate.
//
// It matters more than it looks. messenger_channels used to carry a CHECK
// constraint restricting transport to slack and telegram; that constraint was
// dropped on the grounds that this Go guard is now the thing standing between
// an admin and a channel that mails an arbitrary external address --
// transport_channel_id is free TEXT, so an email channel delivers wherever its
// creator typed.
//
// A test of NotifyTransport.IsValid() alone proves the rule is right and never
// that anyone calls it: deleting this guard leaves such a test green. So this
// one goes through CreateChannel.
func TestCreateChannel_TransportGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cmd := func(transport entity.NotifyTransport) *entity.CreateNotifyChannelCmd {
		return &entity.CreateNotifyChannelCmd{
			Transport:          transport,
			TransportChannelID: t.Name() + "-" + xuuid.NewString(),
			Name:               t.Name() + "-" + xuuid.NewString(),
			Description:        "guard test",
			CreatedByUserID:    uuid.New(),
		}
	}

	// email is a real delivery transport -- invitations and one-time codes go
	// out over it -- but never a channel anyone may subscribe to.
	_, err := svc.CreateChannel(ctx, cmd(entity.NotifyTransportEmail))
	require.ErrorIs(t, err, apperr.ErrValidation,
		"an email channel would deliver to whatever address its creator typed")

	// The stub is wired by configuration, not chosen by an operator.
	_, err = svc.CreateChannel(ctx, cmd(entity.NotifyTransportStub))
	require.ErrorIs(t, err, apperr.ErrValidation)

	_, err = svc.CreateChannel(ctx, cmd("smtp"))
	require.ErrorIs(t, err, apperr.ErrValidation)

	// And the other half, in the same test so it cannot pass by refusing
	// everything: the two subscribable transports still go through.
	for _, transport := range []entity.NotifyTransport{
		entity.NotifyTransportSlack,
		entity.NotifyTransportTelegram,
	} {
		created, createErr := svc.CreateChannel(ctx, cmd(transport))
		require.NoErrorf(t, createErr, "%s must still be creatable", transport)
		require.Equal(t, transport, created.Transport)

		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM messenger_channels WHERE id = $1`, created.ID)
		})
	}
}

package notifytargets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestListByMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	t.Run("resolves catalog channel data", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)
		channel := makeChannel(ctx, t, entity.NotifyTransportTelegram)

		created, err := store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{
			{ChannelID: channel.ID},
		})
		require.NoError(t, err)
		require.Len(t, created, 1)

		targets, err := store.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Len(t, targets, 1)

		target := targets[0]
		require.Equal(t, created[0].ID, target.ID)
		require.Equal(t, maint.ID, target.MaintID)
		require.Equal(t, channel.ID, target.ChannelID)
		require.Equal(t, channel.Transport, target.Transport)
		require.Equal(t, channel.TransportChannelID, target.TransportChannelID)
		require.Equal(t, channel.Name, target.ChannelName)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		got, err := store.ListByMaint(ctx, xuuid.New())
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

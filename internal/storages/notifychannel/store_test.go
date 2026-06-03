package notifychannel

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestStore_Create_AssignsUUIDAndActive(t *testing.T) {
	ctx := context.Background()

	channel := makeChannel(ctx, t, entity.NotifyTransportSlack)

	require.NotEqual(t, uuid.Nil, channel.ID)
	require.Nil(t, channel.ArchivedAt, "freshly created channel must be active")
	require.Equal(t, entity.NotifyTransportSlack, channel.Transport)
}

func TestStore_Create_DuplicateIsConflict(t *testing.T) {
	ctx := context.Background()

	channel := makeChannel(ctx, t, entity.NotifyTransportSlack)

	_, err := store.Create(ctx, &entity.NotifyChannel{
		Transport:          channel.Transport,
		TransportChannelID: channel.TransportChannelID,
		Name:               "dup",
		Description:        "dup",
	})
	require.ErrorIs(t, err, apperr.ErrNotifyChannelAlreadyExists)
}

func TestStore_Get_ByUUID(t *testing.T) {
	ctx := context.Background()

	channel := makeChannel(ctx, t, entity.NotifyTransportTelegram)

	got, err := store.Get(ctx, channel.ID)
	require.NoError(t, err)
	require.Equal(t, channel.ID, got.ID)
	require.Equal(t, channel.TransportChannelID, got.TransportChannelID)
	require.Equal(t, channel.Transport, got.Transport)
}

func TestStore_Get_UnknownIsNotFound(t *testing.T) {
	ctx := context.Background()

	_, err := store.Get(ctx, uuid.New())
	require.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestStore_List_HidesArchivedByDefault(t *testing.T) {
	ctx := context.Background()

	channel := makeChannel(ctx, t, entity.NotifyTransportSlack)
	require.NoError(t, store.Archive(ctx, channel.ID))

	active, err := store.List(ctx, false)
	require.NoError(t, err)
	require.False(t, containsID(active, channel.ID), "archived channel must be hidden from default List")

	all, err := store.List(ctx, true)
	require.NoError(t, err)
	require.True(t, containsID(all, channel.ID), "archived channel must appear when includeArchived=true")
}

func TestStore_List_OrderedByTransportThenChannelID(t *testing.T) {
	ctx := context.Background()

	makeChannel(ctx, t, entity.NotifyTransportSlack)

	channels, err := store.List(ctx, true)
	require.NoError(t, err)

	for i := 1; i < len(channels); i++ {
		prev, cur := channels[i-1], channels[i]
		prevKey := string(prev.Transport) + ":" + prev.TransportChannelID
		curKey := string(cur.Transport) + ":" + cur.TransportChannelID
		require.LessOrEqual(t, prevKey, curKey,
			"List must be ordered by (transport, transport_channel_id)")
	}
}

func TestStore_Archive_Unarchive_RoundTrip(t *testing.T) {
	ctx := context.Background()

	channel := makeChannel(ctx, t, entity.NotifyTransportTelegram)

	require.NoError(t, store.Archive(ctx, channel.ID))
	archived, err := store.Get(ctx, channel.ID)
	require.NoError(t, err)
	require.NotNil(t, archived.ArchivedAt, "archived channel must carry archived_at")

	require.NoError(t, store.Unarchive(ctx, channel.ID))
	active, err := store.Get(ctx, channel.ID)
	require.NoError(t, err)
	require.Nil(t, active.ArchivedAt, "unarchived channel must clear archived_at")
}

func TestStore_Archive_UnknownIsNoop(t *testing.T) {
	ctx := context.Background()

	// Idempotent: archiving/unarchiving an unknown id updates zero rows and
	// is a success, mirroring resource archive semantics.
	require.NoError(t, store.Archive(ctx, uuid.New()))
	require.NoError(t, store.Unarchive(ctx, uuid.New()))
}

func containsID(channels []*entity.NotifyChannel, id uuid.UUID) bool {
	for _, ch := range channels {
		if ch.ID == id {
			return true
		}
	}
	return false
}

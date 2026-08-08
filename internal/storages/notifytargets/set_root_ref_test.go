package notifytargets

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestSetRootRef(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	t.Run("writes the root and ListByMaint reads it back", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)
		channel := makeChannel(ctx, t, entity.NotifyTransportSlack)

		_, err := store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{{ChannelID: channel.ID}})
		require.NoError(t, err)

		ref := entity.MessageRef{MessageID: "1503435956.000247"}
		require.NoError(t, store.SetRootRef(ctx, maint.ID, channel.ID, ref.MessageID, "#ops"))

		targets, err := store.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Len(t, targets, 1)
		require.NotNil(t, targets[0].RootRef)
		require.Equal(t, ref, *targets[0].RootRef)
		// The delivery address is what the re-point guard compares against.
		require.Equal(t, "#ops", targets[0].RootChannel)
	})

	// The key is the (maintenance_id, channel_id) pair, not the row id: row ids
	// do not survive Replace. Two targets of the same maintenance must therefore
	// not overwrite each other, and each channel keeps its own root.
	t.Run("keys by the maintenance and channel pair", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)
		targets := makeNotifyTargets(ctx, t, store, maint.ID)
		require.Len(t, targets, 2)

		first := entity.MessageRef{MessageID: "11"}
		second := entity.MessageRef{MessageID: "22"}
		require.NoError(t, store.SetRootRef(ctx, maint.ID, targets[0].ChannelID, first.MessageID, "chan-addr"))
		require.NoError(t, store.SetRootRef(ctx, maint.ID, targets[1].ChannelID, second.MessageID, "chan-addr"))

		got, err := store.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Len(t, got, 2)

		byChannel := map[string]*entity.MessageRef{}
		for _, target := range got {
			byChannel[target.ChannelID.String()] = target.RootRef
		}

		require.NotNil(t, byChannel[targets[0].ChannelID.String()])
		require.Equal(t, first, *byChannel[targets[0].ChannelID.String()])
		require.NotNil(t, byChannel[targets[1].ChannelID.String()])
		require.Equal(t, second, *byChannel[targets[1].ChannelID.String()])
	})

	// The subscription can be removed between the send and the root write. That
	// updates zero rows, which is not an error: there is nothing left to anchor.
	t.Run("missing target is not an error", func(t *testing.T) {
		t.Parallel()

		err := store.SetRootRef(ctx, xuuid.New(), xuuid.New(), "1", "chan-addr")
		require.NoError(t, err)
	})

	// Two lifecycle events can reach the store at once — dispatch holds no lock
	// and the write is a bare UPDATE. Whichever lands second wins, and the row
	// must end up holding one internally consistent reference rather than a mix
	// of the two. Interleaving is what a partial column write would look like.
	t.Run("concurrent writes leave one consistent root", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)
		channel := makeChannel(ctx, t, entity.NotifyTransportSlack)

		_, err := store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{{ChannelID: channel.ID}})
		require.NoError(t, err)

		first := entity.MessageRef{MessageID: "111"}
		second := entity.MessageRef{MessageID: "222"}

		var wg sync.WaitGroup

		wg.Add(2)

		errs := make([]error, 2)

		go func() {
			defer wg.Done()

			errs[0] = store.SetRootRef(ctx, maint.ID, channel.ID, first.MessageID, "addr-first")
		}()
		go func() {
			defer wg.Done()

			errs[1] = store.SetRootRef(ctx, maint.ID, channel.ID, second.MessageID, "addr-second")
		}()
		wg.Wait()

		require.NoError(t, errs[0])
		require.NoError(t, errs[1])

		targets, err := store.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Len(t, targets, 1)
		require.NotNil(t, targets[0].RootRef, "a concurrent write must not leave the root half-written")

		// The winner may be either, but the row must be entirely one of them:
		// a chat id from one write paired with a message id from the other
		// would point at a message that does not exist.
		switch targets[0].RootRef.MessageID {
		case first.MessageID:
			require.Equal(t, first, *targets[0].RootRef)
			require.Equal(t, "addr-first", targets[0].RootChannel)
		case second.MessageID:
			require.Equal(t, second, *targets[0].RootRef)
			require.Equal(t, "addr-second", targets[0].RootChannel)
		default:
			t.Fatalf("root is neither write: %+v", *targets[0].RootRef)
		}
	})

	t.Run("does not touch other maintenances subscribed to the same channel", func(t *testing.T) {
		t.Parallel()

		channel := makeChannel(ctx, t, entity.NotifyTransportSlack)
		mine := makeMaint(ctx, t)
		other := makeMaint(ctx, t)

		_, err := store.CreateMany(ctx, mine.ID, []*entity.NotifyTarget{{ChannelID: channel.ID}})
		require.NoError(t, err)
		_, err = store.CreateMany(ctx, other.ID, []*entity.NotifyTarget{{ChannelID: channel.ID}})
		require.NoError(t, err)

		ref := entity.MessageRef{MessageID: "99"}
		require.NoError(t, store.SetRootRef(ctx, mine.ID, channel.ID, ref.MessageID, "chan-addr"))

		otherTargets, err := store.ListByMaint(ctx, other.ID)
		require.NoError(t, err)
		require.Len(t, otherTargets, 1)
		require.Nil(t, otherTargets[0].RootRef)
	})
}

func TestSetRootRefOverwritesARejectedRoot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	maint := makeMaint(ctx, t)
	channel := makeChannel(ctx, t, entity.NotifyTransportSlack)

	_, err := store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{{ChannelID: channel.ID}})
	require.NoError(t, err)

	require.NoError(t, store.SetRootRef(ctx, maint.ID, channel.ID, "42", "chan-addr"))
	// The transport rejected 42, so the message it managed to deliver becomes
	// the new root — the remaining events group under it instead of scattering.
	require.NoError(t, store.SetRootRef(ctx, maint.ID, channel.ID, "77", "chan-addr"))

	targets, err := store.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	require.NotNil(t, targets[0].RootRef)
	require.Equal(t, "77", targets[0].RootRef.MessageID)
}

// The root lives on the target row, so removing the subscription removes the
// root with it. That invariant is why the data lives here rather than on the
// maintenance, and Replace (delete-all + create-all) relies on it.
func TestDeleteDropsRootRef(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	maint := makeMaint(ctx, t)
	channel := makeChannel(ctx, t, entity.NotifyTransportSlack)

	_, err := store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{{ChannelID: channel.ID}})
	require.NoError(t, err)
	require.NoError(t, store.SetRootRef(ctx, maint.ID, channel.ID, "7", "chan-addr"))

	require.NoError(t, store.Delete(ctx, maint.ID))

	_, err = store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{{ChannelID: channel.ID}})
	require.NoError(t, err)

	targets, err := store.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	require.Nil(t, targets[0].RootRef, "re-created target must not inherit the deleted row's root")
}

// A missing or empty root_message_id degrades to nil: it is what every
// transport addresses a reply with, so without it there is nothing to reply to.
func TestRootRefFromDBRejectsMissingMessageID(t *testing.T) {
	t.Parallel()

	empty := ""
	value := "42"

	cases := []struct {
		name      string
		messageID *string
	}{
		{name: "nil"},
		{name: "empty string", messageID: &empty},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := rootRefFromDB(&model.MaintenanceNotifyTargets{RootMessageID: tc.messageID})
			require.Nil(t, got)
		})
	}

	t.Run("present", func(t *testing.T) {
		t.Parallel()

		got := rootRefFromDB(&model.MaintenanceNotifyTargets{RootMessageID: &value})
		require.NotNil(t, got)
		require.Equal(t, entity.MessageRef{MessageID: value}, *got)
	})
}

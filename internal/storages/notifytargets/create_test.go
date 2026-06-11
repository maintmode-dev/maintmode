package notifytargets

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestCreateMany(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)
		slackChan := makeChannel(ctx, t, entity.NotifyTransportSlack)
		telegramChan := makeChannel(ctx, t, entity.NotifyTransportTelegram)

		targets, err := store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{
			{ChannelID: slackChan.ID},
			{ChannelID: telegramChan.ID},
		})
		require.NoError(t, err)
		require.Len(t, targets, 2)

		dbTargets, err := store.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Len(t, dbTargets, 2)

		actualTargets := lo.SliceToMap(targets, func(item *entity.NotifyTarget) (uuid.UUID, *entity.NotifyTarget) {
			return item.ID, item
		})

		// ListByMaint enriches targets with catalog data while CreateMany
		// returns the persisted columns only — compare those.
		for _, dbTarget := range dbTargets {
			actual, ok := actualTargets[dbTarget.ID]
			require.True(t, ok, "not found")
			require.Equal(t, dbTarget.MaintID, actual.MaintID)
			require.Equal(t, dbTarget.ChannelID, actual.ChannelID)
			require.Equal(t, dbTarget.CreatedAt, actual.CreatedAt)
		}
	})

	t.Run("unknown channel violates the catalog FK", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)

		_, err := store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{
			{ChannelID: uuid.New()},
		})
		require.Error(t, err)
	})

	t.Run("duplicate", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)

		targets := makeNotifyTargets(ctx, t, store, maint.ID)

		_, err := store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{targets[0]})
		require.Error(t, err)
		require.ErrorIs(t, err, apperr.ErrNotifyTargetsAlreadyExists)
	})
}

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

		targets, err := store.CreateMany(ctx, maint.ID, []*entity.NotifyTarget{
			{Transport: entity.NotifyTransportSlack, ChannelID: "C123"},
			{Transport: entity.NotifyTransportTelegram, ChannelID: "-1001"},
		})
		require.NoError(t, err)
		require.Len(t, targets, 2)

		dbTargets, err := store.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Len(t, dbTargets, 2)

		actualTargets := lo.SliceToMap(targets, func(item *entity.NotifyTarget) (uuid.UUID, *entity.NotifyTarget) {
			return item.ID, item
		})

		for _, dbTarget := range dbTargets {
			actual, ok := actualTargets[dbTarget.ID]
			require.True(t, ok, "not found")
			require.Equal(t, dbTarget, actual)
		}
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

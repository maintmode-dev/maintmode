package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestDeleteMentions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)

	t.Run("removes only the maintenance own mentions", func(t *testing.T) {
		t.Parallel()

		period := entity.NewPeriod(now, now.Add(time.Minute))
		maint := makeMaint(ctx, t, store, period)
		other := makeMaint(ctx, t, store, period)

		otherUserID := xuuid.New()
		require.NoError(t, store.AddMentions(ctx, maint.ID, []uuid.UUID{xuuid.New(), xuuid.New()}))
		require.NoError(t, store.AddMentions(ctx, other.ID, []uuid.UUID{otherUserID}))

		require.NoError(t, store.DeleteMentions(ctx, maint.ID))

		got, err := store.GetMaintMentions(ctx, maint.ID)
		require.NoError(t, err)
		require.Empty(t, got)

		otherGot, err := store.GetMaintMentions(ctx, other.ID)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{otherUserID}, otherGot)
	})

	t.Run("no mentions is not an error", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))
		require.NoError(t, store.DeleteMentions(ctx, maint.ID))
	})
}

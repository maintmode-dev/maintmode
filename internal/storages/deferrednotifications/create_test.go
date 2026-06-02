package deferrednotifications

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestCreateManyAndList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)

		created, err := store.CreateMany(ctx, maint.ID, sampleNotifications(xtime.UTCNow()))
		require.NoError(t, err)
		require.Len(t, created, 2)
		// ids assigned, no task id until enqueue
		for _, n := range created {
			require.NotEqual(t, uuid.Nil, n.ID)
			require.Nil(t, n.GoqueTaskID)
		}

		listed, err := store.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Len(t, listed, 2)
		// ordered by fire_at asc
		require.True(t, !listed[1].FireAt.Before(listed[0].FireAt))
	})

	t.Run("empty is no-op", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t)

		created, err := store.CreateMany(ctx, maint.ID, nil)
		require.NoError(t, err)
		require.Empty(t, created)

		listed, err := store.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Empty(t, listed)
	})
}

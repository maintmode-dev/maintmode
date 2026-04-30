package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestDeleteResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))

		maintResources, err := store.GetMaintResources(ctx, []uuid.UUID{maint.ID})
		require.NoError(t, err)

		resources := maintResources[maint.ID]
		require.Equal(t, len(maint.Resources), len(resources))

		err = store.DeleteResources(ctx, maint.ID)
		require.NoError(t, err)

		maintResources, err = store.GetMaintResources(ctx, []uuid.UUID{maint.ID})
		require.NoError(t, err)
		require.Equal(t, 0, len(maintResources))
	})
}

package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestDeleteSteps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))

		steps := makeSteps(ctx, t, store, maint.ID)

		maintSteps, err := store.GetMaintSteps(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, len(steps), len(maintSteps))

		err = store.DeleteSteps(ctx, maint.ID)
		require.NoError(t, err)

		maintSteps, err = store.GetMaintSteps(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, 0, len(maintSteps))
	})
}

package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestAddResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))
		resource := makeResource(ctx, t, entity.ResourceTypeService)

		err := store.AddResources(ctx, maint.ID, []*entity.Resource{resource})
		require.NoError(t, err)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))

		err := store.AddResources(ctx, maint.ID, []*entity.Resource{})
		require.NoError(t, err)
	})
}

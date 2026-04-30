package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	now := xtime.UTCNow()
	start, end := now.Add(time.Hour), now.Add(2*time.Hour)
	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(start, end))
		maint.Description = "New description"

		err := store.UpdateMaint(ctx, maint)
		require.NoError(t, err)

		dbMaint, err := store.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.True(t, dbMaint.UpdatedAt.After(xtime.UTCNow().Add(-time.Second)))

		maint.Resources = nil
		maint.UpdatedAt = dbMaint.UpdatedAt
		require.Equal(t, maint, dbMaint)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		originalMaint := makeMaint(ctx, t, store, entity.NewPeriod(start, end))

		maint := *originalMaint
		maint.ID = xuuid.New()
		maint.Description = "New description"

		err := store.UpdateMaint(ctx, &maint)
		require.NoError(t, err)

		dbMaint, err := store.GetMaint(ctx, originalMaint.ID)
		require.NoError(t, err)
		originalMaint.Resources = nil
		require.Equal(t, originalMaint, dbMaint)
	})
}

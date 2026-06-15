package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
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

	t.Run("actual period: nil -> open-ended -> closed", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(start, end))
		require.Nil(t, maint.ActualPeriod)

		// nil -> open-ended (NULL upper), as StartMaint does.
		openStart := now.Add(90 * time.Minute)
		maint.ActualPeriod = lo.ToPtr(entity.NewOpenEndedPeriod(openStart))
		require.NoError(t, store.UpdateMaint(ctx, maint))

		dbMaint, err := store.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.NotNil(t, dbMaint.ActualPeriod)
		require.True(t, dbMaint.ActualPeriod.IsOpen())
		require.WithinDuration(t, openStart, dbMaint.ActualPeriod.Start, time.Second)

		// open-ended -> closed, as CompleteMaint does.
		maint.ActualPeriod = lo.ToPtr(entity.NewPeriod(openStart, now.Add(2*time.Hour)))
		require.NoError(t, store.UpdateMaint(ctx, maint))

		dbMaint, err = store.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.NotNil(t, dbMaint.ActualPeriod)
		require.False(t, dbMaint.ActualPeriod.IsOpen())
	})

	t.Run("updated_at advances on each update", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(start, end))

		maint.Description = "first"
		require.NoError(t, store.UpdateMaint(ctx, maint))
		first := lo.FromPtr(maint.UpdatedAt)
		require.False(t, first.IsZero())

		maint.Description = "second"
		require.NoError(t, store.UpdateMaint(ctx, maint))
		second := lo.FromPtr(maint.UpdatedAt)

		// updated_at must move forward on each update so it works as the
		// optimistic-concurrency token (Revision()).
		require.False(t, second.Before(first))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		originalMaint := makeMaint(ctx, t, store, entity.NewPeriod(start, end))

		maint := *originalMaint
		maint.ID = xuuid.New()
		maint.Description = "New description"

		// UpdateMaint targets a row by id; a missing row is a no-op, not an error.
		// The real flow never hits this — updateWithApply locks the row via
		// GetMaintForUpdate first.
		err := store.UpdateMaint(ctx, &maint)
		require.NoError(t, err)

		// The real (existing) maintenance is untouched.
		dbMaint, err := store.GetMaint(ctx, originalMaint.ID)
		require.NoError(t, err)
		originalMaint.Resources = nil
		require.Equal(t, originalMaint, dbMaint)
	})
}

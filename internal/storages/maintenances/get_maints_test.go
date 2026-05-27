package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/calendardto"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)
	start, end := now.Add(time.Hour), now.Add(2*time.Hour)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		// for example
		// maint1 11:00 to 13:00
		// maint2 13:00 to 14:00
		// maint3 10:00 to 14:00
		maint1 := makeMaint(ctx, t, store, entity.NewPeriod(start, end))
		maint2 := makeMaint(ctx, t, store, entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)))
		maint3 := makeMaint(ctx, t, store, entity.NewPeriod(start.Add(-time.Hour), end.Add(time.Hour)))

		// try to found maints in the period: 11:20 to 13:20
		maints, truncated, err := store.GetMaints(ctx, &calendardto.GetMaintsFilter{
			PeriodFrom: start.Add(20 * time.Minute),
			PeriodTo:   end.Add(20 * time.Minute),
		}, 100_000)
		require.NoError(t, err)
		require.False(t, truncated)
		require.GreaterOrEqual(t, len(maints), 3)
		equalMaintsWithoutResources(t, maints, []*entity.Maintenance{maint1, maint2, maint3})
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		maints, truncated, err := store.GetMaints(ctx, &calendardto.GetMaintsFilter{
			PeriodFrom: start.Add(365 * 10 * time.Hour),
			PeriodTo:   start.Add(365 * 10 * time.Hour),
		}, 1000)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Equal(t, len(maints), 0)
	})

	t.Run("truncated", func(t *testing.T) {
		t.Parallel()

		makeMaint(ctx, t, store, entity.NewPeriod(start, end))
		makeMaint(ctx, t, store, entity.NewPeriod(start, end))
		makeMaint(ctx, t, store, entity.NewPeriod(start, end))

		maints, truncated, err := store.GetMaints(ctx, &calendardto.GetMaintsFilter{
			PeriodFrom: start,
			PeriodTo:   end,
		}, 1)
		require.NoError(t, err)
		require.True(t, truncated)
		require.Equal(t, len(maints), 1)
	})

	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()

		// for example
		// maint1 10:00 to 11:00 = [10:00, 11:00)
		// maint2 11:00 to 12:00 = [11:00, 12:00)
		maint1 := makeMaint(ctx, t, store, entity.NewPeriod(start, end))
		maint2 := makeMaint(ctx, t, store, entity.NewPeriod(
			maint1.PlannedPeriod.Start.Add(time.Hour),
			maint1.PlannedPeriod.End.Add(time.Hour)),
		)

		maints, truncated, err := store.GetMaints(ctx, &calendardto.GetMaintsFilter{
			PeriodFrom: maint2.PlannedPeriod.Start,
		}, 100_000)
		require.NoError(t, err)
		require.False(t, truncated)
		require.GreaterOrEqual(t, len(maints), 1)

		equalMaintsWithoutResources(t, maints, []*entity.Maintenance{maint2})
		require.NotContains(t, maints, maint1)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		maints, truncated, err := store.GetMaints(ctx, &calendardto.GetMaintsFilter{
			PeriodFrom:  start,
			PeriodTo:    end,
			ResourceIDs: []uuid.UUID{xuuid.New()},
		}, 1)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Equal(t, len(maints), 0)
	})

	t.Run("with resources", func(t *testing.T) {
		t.Parallel()

		maint := makeMaint(ctx, t, store, entity.NewPeriod(start, end))

		maints, truncated, err := store.GetMaints(ctx, &calendardto.GetMaintsFilter{
			PeriodFrom:  start,
			PeriodTo:    end,
			ResourceIDs: maint.Resources,
		}, 1)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Equal(t, len(maints), 1)
	})
}

func equalMaintsWithoutResources(t *testing.T, actual, expected []*entity.Maintenance) {
	t.Helper()

	actualMap := lo.SliceToMap(actual, func(item *entity.Maintenance) (uuid.UUID, *entity.Maintenance) {
		return item.ID, item
	})

	for _, exp := range expected {
		act, ok := actualMap[exp.ID]
		require.Truef(t, ok, "expected maint with id %s [period: %v]", exp.ID, exp.PlannedPeriod)
		exp.Resources = nil
		require.Equal(t, exp, act)
	}
}

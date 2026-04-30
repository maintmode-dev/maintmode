package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestAddSteps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))

		step := &entity.MaintenanceStep{
			Order:               1,
			Description:         "Description" + t.Name(),
			RollbackDescription: "RollbackDescription",
			DurationMinutes:     1,
			Status:              entity.MaintenanceStepStatusPlanned,
		}
		_, err := store.AddSteps(ctx, maint.ID, []*entity.MaintenanceStep{step})
		require.NoError(t, err)

		steps, err := store.GetMaintSteps(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, 1, len(steps))
		require.Equal(t, step.Description, steps[0].Description)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		maint := makeMaint(ctx, t, store, entity.NewPeriod(now, now.Add(time.Minute)))

		_, err := store.AddSteps(ctx, maint.ID, []*entity.MaintenanceStep{})
		require.NoError(t, err)
	})
}

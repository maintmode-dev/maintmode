package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestMaintCloseActualPeriod(t *testing.T) {
	start := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	t.Run("nil actual period stays nil", func(t *testing.T) {
		maint := &Maintenance{}

		maint.CloseActualPeriod(start)

		require.Nil(t, maint.ActualPeriod)
	})

	t.Run("closes a started period", func(t *testing.T) {
		maint := &Maintenance{ActualPeriod: lo.ToPtr(NewOpenEndedPeriod(start))}
		end := start.Add(time.Hour)

		maint.CloseActualPeriod(end)

		require.NotNil(t, maint.ActualPeriod)
		require.Equal(t, start, maint.ActualPeriod.Start)
		require.Equal(t, end, lo.FromPtr(maint.ActualPeriod.End))
	})

	t.Run("drops a period that has not started yet", func(t *testing.T) {
		// A start in the future would produce an inverted range on write; the
		// maintenance never actually ran, so the period is dropped.
		maint := &Maintenance{ActualPeriod: lo.ToPtr(NewPeriod(start, start.Add(time.Hour)))}

		maint.CloseActualPeriod(start.Add(-30 * time.Minute))

		require.Nil(t, maint.ActualPeriod)
	})

	t.Run("drops a period starting exactly at the close instant", func(t *testing.T) {
		// lower < upper is strict in the DB constraint, so a zero-length period
		// is dropped too.
		maint := &Maintenance{ActualPeriod: lo.ToPtr(NewOpenEndedPeriod(start))}

		maint.CloseActualPeriod(start)

		require.Nil(t, maint.ActualPeriod)
	})
}

func TestMaintClone(t *testing.T) {
	maint := &Maintenance{
		ID:     uuid.New(),
		Status: MaintenanceStatusPlanned,
	}

	clonedMaint := maint.Clone()

	require.Equal(t, maint.ID, clonedMaint.ID)
	require.Equal(t, maint.Status, clonedMaint.Status)
	require.NotSame(t, maint, clonedMaint)

	maint.Status = MaintenanceStatusCompleted
	require.Equal(t, MaintenanceStatusPlanned, clonedMaint.Status)
}

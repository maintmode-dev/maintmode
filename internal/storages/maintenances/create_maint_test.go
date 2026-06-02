package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/entity"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestCreate_PlannedPeriod(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow().Round(time.Microsecond)

	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		period := entity.NewPeriod(now.Add(-time.Hour), now.Add(time.Hour))
		maint := &entity.Maintenance{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: period,
			Scope:         entity.MaintenanceScopeResources,
			Status:        entity.MaintenanceStatusPlanned,
			Impact:        entity.MaintenanceImpactFull,
		}

		created, err := store.CreateMaint(ctx, maint)
		require.NoError(t, err)
		require.NotNil(t, created)
		equalMaint(t, maint, created)

		dbMaint, err := store.GetMaint(ctx, created.ID)
		require.NoError(t, err)
		require.Equal(t, maint, dbMaint)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name        string
			period      entity.Period
			expectedErr string
		}{
			{
				name:        "start == end",
				period:      entity.NewPeriod(now, now),
				expectedErr: "jet: pq: new row for relation \"maintenances\" violates check constraint \"maintenances_planned_period_check\"",
			}, {
				name:        "start > end",
				period:      entity.NewPeriod(now.Add(time.Hour), now),
				expectedErr: "jet: pq: range lower bound must be less than or equal to range upper bound",
			}, {
				name:        "open-ended",
				period:      entity.NewOpenEndedPeriod(now.Add(time.Hour)),
				expectedErr: "jet: pq: new row for relation \"maintenances\" violates check constraint \"maintenances_planned_period_check\"",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				maint := &entity.Maintenance{
					Title:         "Title" + t.Name(),
					Description:   "Description" + t.Name(),
					PlannedPeriod: tc.period,
					ActualPeriod:  nil,
					Status:        entity.MaintenanceStatusPlanned,
					Impact:        entity.MaintenanceImpactFull,
				}

				created, err := store.CreateMaint(ctx, maint)
				// ErrorContains, not EqualError: pq now appends the SQLSTATE
				// code (e.g. " (23514)") to the message, so match the prefix.
				require.ErrorContains(t, err, tc.expectedErr)
				require.Nil(t, created)

				dbMaint, err := store.GetMaint(ctx, maint.ID)
				require.EqualError(t, err, apperr.ErrMaintNotFound.Error())
				require.Nil(t, dbMaint)
			})
		}
	})
}

func TestCreate_ActualPeriod(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow().Round(time.Microsecond)

	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name         string
			actualPeriod entity.Period
		}{
			{
				name:         "open-ended",
				actualPeriod: entity.NewOpenEndedPeriod(now.Add(-time.Hour)),
			}, {
				name:         "with end",
				actualPeriod: entity.NewPeriod(now.Add(-time.Hour), now.Add(time.Hour)),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				period := entity.NewPeriod(now.Add(-time.Hour), now.Add(time.Hour))
				maint := &entity.Maintenance{
					Title:         "Title" + t.Name(),
					Description:   "Description" + t.Name(),
					PlannedPeriod: period,
					ActualPeriod:  lo.ToPtr(tc.actualPeriod),
					Status:        entity.MaintenanceStatusPlanned,
					Impact:        entity.MaintenanceImpactFull,
				}

				created, err := store.CreateMaint(ctx, maint)
				require.NoError(t, err)
				require.NotNil(t, created)
				equalMaint(t, maint, created)

				dbMaint, err := store.GetMaint(ctx, created.ID)
				require.NoError(t, err)
				require.Equal(t, lo.ToPtr(tc.actualPeriod), dbMaint.ActualPeriod)
				require.Equal(t, created, dbMaint)
			})
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name        string
			period      entity.Period
			expectedErr string
		}{
			{
				name:        "start == end",
				period:      entity.NewPeriod(now, now),
				expectedErr: "jet: pq: new row for relation \"maintenances\" violates check constraint \"maintenances_actual_period_check\"",
			}, {
				name:        "start > end",
				period:      entity.NewPeriod(now.Add(time.Hour), now),
				expectedErr: "jet: pq: range lower bound must be less than or equal to range upper bound",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				maint := &entity.Maintenance{
					ID:            xuuid.New(),
					Title:         "Title" + t.Name(),
					Description:   "Description" + t.Name(),
					PlannedPeriod: entity.NewPeriod(now.Add(-time.Hour), now.Add(time.Hour)),
					ActualPeriod:  lo.ToPtr(tc.period),
					Status:        entity.MaintenanceStatusPlanned,
					Impact:        entity.MaintenanceImpactFull,
					CreatedAt:     now,
				}

				created, err := store.CreateMaint(ctx, maint)
				// ErrorContains, not EqualError: pq now appends the SQLSTATE
				// code (e.g. " (23514)") to the message, so match the prefix.
				require.ErrorContains(t, err, tc.expectedErr)
				require.Nil(t, created)

				dbMaint, err := store.GetMaint(ctx, maint.ID)
				require.EqualError(t, err, apperr.ErrMaintNotFound.Error())
				require.Nil(t, dbMaint)
			})
		}
	})
}

func equalMaint(t *testing.T, expected, actual *entity.Maintenance) {
	t.Helper()

	require.NotEmpty(t, actual.ID)
	require.True(t, actual.CreatedAt.After(xtime.UTCNow().Add(-time.Minute)))

	expected.ID = actual.ID
	expected.CreatedAt = actual.CreatedAt
	require.Equal(t, expected, actual)
}

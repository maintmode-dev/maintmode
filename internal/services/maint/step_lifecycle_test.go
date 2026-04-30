package maint

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	testdbutils "github.com/ruko1202/maintmode/test/utils/db"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestStepLifeCycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := xtime.UTCNow()

	s := initService(db)

	t.Run("StartStep", func(t *testing.T) {
		t.Parallel()

		maint := testdbutils.MakeMaint(ctx, t, s.maintStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusInProgress),
			testdbutils.WithSteps([]*entity.MaintenanceStep{{
				Order:               1,
				Description:         "step1" + t.Name(),
				RollbackDescription: "rollback step1" + t.Name(),
				Status:              entity.MaintenanceStepStatusPlanned,
			}}),
		)

		err := s.StartStep(ctx, &entity.StartMaintenanceStepCmd{
			MaintID: maint.ID,
			StepID:  maint.Steps[0].ID,
		})
		require.NoError(t, err)

		maint, err = s.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStepStatusStarted, maint.Steps[0].Status)
		for _, step := range maint.Steps[1:] {
			require.Equal(t, entity.MaintenanceStepStatusPlanned, step.Status)
		}
	})

	t.Run("CompleteStep", func(t *testing.T) {
		t.Parallel()

		maint := testdbutils.MakeMaint(ctx, t, s.maintStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusInProgress),
			testdbutils.WithSteps([]*entity.MaintenanceStep{{
				Order:               1,
				Description:         "step1" + t.Name(),
				RollbackDescription: "rollback step1" + t.Name(),
				Status:              entity.MaintenanceStepStatusStarted,
			}}),
		)

		err := s.CompleteStep(ctx, &entity.CompleteMaintenanceStepCmd{
			MaintID: maint.ID,
			StepID:  maint.Steps[0].ID,
		})
		require.NoError(t, err)

		maint, err = s.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStepStatusCompleted, maint.Steps[0].Status)
		for _, step := range maint.Steps[1:] {
			require.Equal(t, entity.MaintenanceStepStatusPlanned, step.Status)
		}
	})

	t.Run("CancelStep", func(t *testing.T) {
		t.Parallel()

		maint := testdbutils.MakeMaint(ctx, t, s.maintStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusInProgress),
			testdbutils.WithSteps([]*entity.MaintenanceStep{{
				Order:               1,
				Description:         "step1" + t.Name(),
				RollbackDescription: "rollback step1" + t.Name(),
				Status:              entity.MaintenanceStepStatusStarted,
			}}),
		)

		err := s.CancelStep(ctx, &entity.CancelMaintenanceStepCmd{
			MaintID: maint.ID,
			StepID:  maint.Steps[0].ID,
		})
		require.NoError(t, err)

		maint, err = s.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStepStatusCanceled, maint.Steps[0].Status)
		for _, step := range maint.Steps[1:] {
			require.Equal(t, entity.MaintenanceStepStatusPlanned, step.Status)
		}
	})
}

func TestStepLifecycle_canChangeStepStatusTo(t *testing.T) {
	t.Parallel()

	id1, id2, id3 := xuuid.New(), xuuid.New(), xuuid.New()
	steps := []*entity.MaintenanceStep{
		{
			Order:  1,
			ID:     id1,
			Status: entity.MaintenanceStepStatusPlanned,
		}, {
			Order:  2,
			ID:     id2,
			Status: entity.MaintenanceStepStatusCompleted,
		}, {
			Order:  3,
			ID:     id3,
			Status: entity.MaintenanceStepStatusPlanned,
		},
	}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		step, err := canChangeStepStatusTo(steps, id1, entity.MaintenanceStepStatusStarted)
		require.NoError(t, err)
		require.Equal(t, steps[0], step)
	})

	t.Run("ErrStepNotFound", func(t *testing.T) {
		t.Parallel()

		step, err := canChangeStepStatusTo(steps, xuuid.New(), entity.MaintenanceStepStatusStarted)
		require.ErrorIs(t, err, apperr.ErrStepNotFound)
		require.Nil(t, step)
	})

	t.Run("ErrStepOrderViolation", func(t *testing.T) {
		t.Parallel()

		step, err := canChangeStepStatusTo(steps, id2, entity.MaintenanceStepStatusStarted)
		require.ErrorIs(t, err, apperr.ErrStepOrderViolation)
		require.Nil(t, step)
	})

	t.Run("ErrForbiddenStepStatusTransition", func(t *testing.T) {
		t.Parallel()

		step, err := canChangeStepStatusTo(steps, id3, entity.MaintenanceStepStatusCompleted)
		require.ErrorIs(t, err, apperr.ErrForbiddenStepStatusTransition)
		require.Nil(t, step)
	})
}

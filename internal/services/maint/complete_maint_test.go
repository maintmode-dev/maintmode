package maint

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

func TestComplete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	service, _ := initService(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore, entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		)

		err := service.StartMaint(ctx, &entity.StartMaintenanceCmd{
			MaintID: maint.ID,
		})
		require.NoError(t, err)

		for _, step := range maint.Steps {
			err = service.StartStep(ctx, &entity.StartMaintenanceStepCmd{MaintID: maint.ID, StepID: step.ID})
			require.NoError(t, err)

			err = service.CompleteStep(ctx, &entity.CompleteMaintenanceStepCmd{MaintID: maint.ID, StepID: step.ID})
			require.NoError(t, err)
		}

		err = service.CompleteMaint(ctx, &entity.CompleteMaintenanceCmd{
			MaintID: maint.ID,
		})
		require.NoError(t, err)

		actualMaint, err := service.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStatusCompleted, actualMaint.Status)
	})

	t.Run("ErrForbiddenStatusTransition", func(t *testing.T) {
		t.Parallel()
		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore, entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		)

		err := service.CompleteMaint(ctx, &entity.CompleteMaintenanceCmd{
			MaintID: maint.ID,
		})
		require.ErrorIs(t, err, apperr.ErrForbiddenMaintStatusTransition)

		actualMaint, err := service.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStatusPlanned, actualMaint.Status)
	})
}

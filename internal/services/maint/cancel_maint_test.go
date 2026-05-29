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

func TestCancel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		for _, status := range []entity.MaintenanceStatus{
			entity.MaintenanceStatusDraft,
			entity.MaintenanceStatusPlanned,
			entity.MaintenanceStatusInProgress,
		} {
			t.Run(string(status), func(t *testing.T) {
				t.Parallel()

				maint := testdbutils.MakeMaint(ctx, t, service.maintStore, resourcesStore, entity.NewPeriod(now, now.Add(time.Hour)),
					testdbutils.WithStatus(status),
				)

				err := service.CancelMaint(ctx, &entity.CancelMaintenanceCmd{
					MaintID: maint.ID,
				})
				require.NoError(t, err)

				actualMaint, err := service.GetMaint(ctx, maint.ID)
				require.NoError(t, err)
				require.Equal(t, entity.MaintenanceStatusCancelled, actualMaint.Status)
			})
		}
	})

	t.Run("ErrForbiddenStatusTransition", func(t *testing.T) {
		t.Parallel()
		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, resourcesStore, entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
		)

		err := service.CancelMaint(ctx, &entity.CancelMaintenanceCmd{
			MaintID: maint.ID,
		})
		require.ErrorIs(t, err, apperr.ErrForbiddenMaintStatusTransition)

		actualMaint, err := service.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStatusCompleted, actualMaint.Status)
	})
}

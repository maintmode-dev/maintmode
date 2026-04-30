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

func TestStart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	s := initService(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		maint := testdbutils.MakeMaint(ctx, t, s.maintStore, resourcesStore, entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		)

		err := s.StartMaint(ctx, &entity.StartMaintenanceCmd{
			MaintID: maint.ID,
		})
		require.NoError(t, err)

		actualMaint, err := s.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStatusInProgress, actualMaint.Status)
	})

	t.Run("ErrForbiddenStatusTransition", func(t *testing.T) {
		t.Parallel()
		maint := testdbutils.MakeMaint(ctx, t, s.maintStore, resourcesStore, entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusDraft),
		)

		err := s.StartMaint(ctx, &entity.StartMaintenanceCmd{
			MaintID: maint.ID,
		})
		require.ErrorIs(t, err, apperr.ErrForbiddenMaintStatusTransition)

		actualMaint, err := s.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStatusDraft, actualMaint.Status)
	})
}

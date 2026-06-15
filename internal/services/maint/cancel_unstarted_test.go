package maint

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

func TestCancelUnStarted(t *testing.T) {
	// Not parallel: CancelUnStarted sweeps every not-started maintenance in the
	// shared DB, so running alongside tests that depend on their draft/planned
	// maints staying as-is would be racy. The maints created here all start well
	// past the grace window, which other tests do not.
	ctx := context.Background()
	now := xtime.UTCNow()

	service, _ := initService(t)

	// Overdue planned and overdue draft both start far in the past (> 15 min
	// grace) → both must be canceled. The very old start makes them sort among the
	// oldest overdue rows, so the sweep's batch limit covers them even when the
	// shared test DB already holds many overdue maints.
	overdueStart := now.Add(-10000 * time.Hour)
	overduePlanned := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
		entity.NewPeriod(overdueStart, overdueStart.Add(time.Hour)),
		testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
	)
	overdueDraft := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
		entity.NewPeriod(overdueStart, overdueStart.Add(time.Hour)),
		testdbutils.WithStatus(entity.MaintenanceStatusDraft),
	)

	// Within grace: planned start is 5 min ago (< 15 min) → must stay planned.
	freshPlanned := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
		entity.NewPeriod(now.Add(-5*time.Minute), now.Add(25*time.Minute)),
		testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
	)

	err := service.CancelUnStarted(ctx, now.Add(-15*time.Minute), 100_000)
	require.NoError(t, err)

	gotPlanned, err := service.GetMaint(ctx, overduePlanned.ID)
	require.NoError(t, err)
	require.Equal(t, entity.MaintenanceStatusCancelled, gotPlanned.Status)
	require.Equal(t, entity.MaintenanceCancelReasonNotStarted, gotPlanned.CancelReason)
	require.NotEmpty(t, gotPlanned.CancelReasonComment)

	gotDraft, err := service.GetMaint(ctx, overdueDraft.ID)
	require.NoError(t, err)
	require.Equal(t, entity.MaintenanceStatusCancelled, gotDraft.Status)
	require.Equal(t, entity.MaintenanceCancelReasonNotStarted, gotDraft.CancelReason)

	gotFresh, err := service.GetMaint(ctx, freshPlanned.ID)
	require.NoError(t, err)
	require.Equal(t, entity.MaintenanceStatusPlanned, gotFresh.Status)
}

// TestCancelOneUnStarted_SkipsStartedOrFinished guards the TOCTOU window: a
// maintenance that was listed as not-started but has changed status by the time
// the per-row cancel runs must be left untouched. in_progress is a valid source
// for a manual cancel, so canceling it here would kill running work and mislabel
// it "not_started". Exercising the per-row helper directly is the faithful test:
// the status the helper observes under the lock stands in for the racing
// transition, since ListOverdueNotStarted only ever returns draft/planned.
func TestCancelOneUnStarted_SkipsStartedOrFinished(t *testing.T) {
	ctx := context.Background()
	now := xtime.UTCNow()

	service, _ := initService(t)

	start := now.Add(-20 * time.Minute)

	for _, status := range []entity.MaintenanceStatus{
		entity.MaintenanceStatusInProgress,
		entity.MaintenanceStatusCompleted,
		entity.MaintenanceStatusCancelled,
	} {
		t.Run(string(status), func(t *testing.T) {
			maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
				entity.NewPeriod(start, start.Add(time.Hour)),
				testdbutils.WithStatus(status),
			)

			err := service.cancelOneUnStarted(ctx, maint.ID)
			require.NoError(t, err)

			got, err := service.GetMaint(ctx, maint.ID)
			require.NoError(t, err)
			require.Equal(t, status, got.Status, "started/finished maintenance must be left untouched")
		})
	}
}

// TestCancelUnStarted_NoOverdue verifies the sweep is a clean no-op when nothing
// qualifies (planned but still within grace).
func TestCancelUnStarted_NoOverdue(t *testing.T) {
	ctx := context.Background()
	now := xtime.UTCNow()

	service, _ := initService(t)

	fresh := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
		entity.NewPeriod(now.Add(-time.Minute), now.Add(time.Hour)),
		testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
	)

	require.NoError(t, service.CancelUnStarted(ctx, now.Add(-15*time.Minute), 100_000))

	got, err := service.GetMaint(ctx, fresh.ID)
	require.NoError(t, err)
	require.Equal(t, entity.MaintenanceStatusPlanned, got.Status)
}

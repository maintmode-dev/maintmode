package maint

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// TestUpdateMaintDeferredNotificationsAtCap pins the accepting side of the cap.
// The rejecting side is covered in TestUpdateDraft, but it asserts the error
// text ("no more than 10") rather than the boundary, so tightening the cap to 9
// stays green there as long as the message is updated with it. Sending exactly
// maxDeferredNotifications and requiring success is what actually fixes the
// boundary in place.
func TestUpdateMaintDeferredNotificationsAtCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	service, _ := initService(t)

	start := xtime.UTCNow().Add(uniqueFutureOffset())
	maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
		entity.NewPeriod(start, start.Add(time.Hour)))

	atCap := lo.RepeatBy(maxDeferredNotifications, func(i int) *entity.DeferredNotificationInput {
		return &entity.DeferredNotificationInput{FireAt: start.Add(-time.Duration(i+1) * time.Minute)}
	})

	err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
		MaintID:               maint.ID,
		Title:                 lo.ToPtr("Updated title"),
		DeferredNotifications: lo.ToPtr(atCap),
		Actor:                 actor(),
	})
	require.NoError(t, err, "exactly maxDeferredNotifications reminders must be accepted")

	got, err := service.deferred.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.Len(t, got, maxDeferredNotifications)
}

// TestUpdateMaintDeferredNotificationsRejectedOnNonDraft guards the status gate
// that makes clearing safe. Edits are draft-only, and the reminder logic leans
// on that: it hard-deletes rows without canceling goque tasks, on the grounds
// that a draft has none enqueued yet. If the gate ever widened to an approved
// maintenance, clearing would delete reminders whose tasks are already
// scheduled. Every other lifecycle command (start/complete/cancel/approve) has
// such a test; update did not.
func TestUpdateMaintDeferredNotificationsRejectedOnNonDraft(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	service, _ := initService(t)

	start := xtime.UTCNow().Add(uniqueFutureOffset())
	// Planned, not draft: this is the state whose reminders are actually
	// enqueued as goque tasks.
	maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
		entity.NewPeriod(start, start.Add(time.Hour)),
		testdbutils.WithStatus(entity.MaintenanceStatusPlanned))

	existing, err := service.deferred.Create(ctx, maint.ID, []*entity.DeferredNotification{
		{MaintID: maint.ID, FireAt: start.Add(-time.Hour)},
	})
	require.NoError(t, err)
	require.Len(t, existing, 1)

	err = service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
		MaintID:               maint.ID,
		Title:                 lo.ToPtr("Updated title"),
		DeferredNotifications: lo.ToPtr([]*entity.DeferredNotificationInput{}),
		Actor:                 actor(),
	})
	require.ErrorIs(t, err, apperr.ErrForbiddenMaintStatusTransition,
		"editing a non-draft maintenance must be rejected")

	got, err := service.deferred.ListByMaint(ctx, maint.ID)
	require.NoError(t, err)
	require.Len(t, got, 1, "a rejected edit must leave the enqueued reminders in place")
}

// TestUpdateMaintDeferredNotificationsTriState pins the clearing contract: a nil
// set leaves reminders alone, an empty set clears them, and a non-empty set
// replaces them. Assertions read the persisted rows back rather than trusting a
// "Replace was called" signal — the empty case must actually delete the previous
// reminders, not rewrite them.
func TestUpdateMaintDeferredNotificationsTriState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	service, mocks := initService(t)

	// seedMaint creates a draft that already carries two reminders.
	seedMaint := func(t *testing.T) (*entity.Maintenance, []*entity.DeferredNotification) {
		t.Helper()

		start := xtime.UTCNow().Add(uniqueFutureOffset())
		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(start, start.Add(time.Hour)))

		existing, err := service.deferred.Create(ctx, maint.ID, []*entity.DeferredNotification{
			{MaintID: maint.ID, FireAt: start.Add(-2 * time.Hour)},
			{MaintID: maint.ID, FireAt: start.Add(-time.Hour)},
		})
		require.NoError(t, err)
		require.Len(t, existing, 2)

		return maint, existing
	}

	t.Run("nil leaves reminders unchanged", func(t *testing.T) {
		t.Parallel()

		maint, _ := seedMaint(t)

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:               maint.ID,
			Title:                 lo.ToPtr("Updated title"),
			DeferredNotifications: nil,
			Actor:                 actor(),
		})
		require.NoError(t, err)

		got, err := service.deferred.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Len(t, got, 2, "a nil set must not touch the existing reminders")
	})

	t.Run("empty set clears reminders", func(t *testing.T) {
		t.Parallel()

		maint, _ := seedMaint(t)

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:               maint.ID,
			Title:                 lo.ToPtr("Updated title"),
			DeferredNotifications: lo.ToPtr([]*entity.DeferredNotificationInput{}),
			Actor:                 actor(),
		})
		require.NoError(t, err)

		got, err := service.deferred.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Empty(t, got, "an empty set must delete the reminders, not rewrite them")
	})

	t.Run("non-empty set replaces reminders", func(t *testing.T) {
		t.Parallel()

		maint, existing := seedMaint(t)
		replacement := xtime.UTCNow().Add(uniqueFutureOffset())

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID: maint.ID,
			Title:   lo.ToPtr("Updated title"),
			DeferredNotifications: lo.ToPtr([]*entity.DeferredNotificationInput{
				{FireAt: replacement},
			}),
			Actor: actor(),
		})
		require.NoError(t, err)

		got, err := service.deferred.ListByMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.WithinDuration(t, replacement, got[0].FireAt, time.Second)
		require.NotEqual(t, existing[0].ID, got[0].ID, "reminders should be replaced, not kept")
	})

	// Pins the staging gate on its own. Replace persists whatever this function
	// stages, so if the two gates disagree an empty set would carry the previous
	// reminders into Replace and rewrite them instead of clearing. The
	// end-to-end assertions above cannot see that today only because the store
	// never hydrates this field; this test does not depend on that.
	t.Run("staging gate mirrors the replace gate", func(t *testing.T) {
		t.Parallel()

		existing := []*entity.DeferredNotification{{FireAt: xtime.UTCNow()}}

		t.Run("empty set stages an empty collection", func(t *testing.T) {
			t.Parallel()

			maint := &entity.Maintenance{DeferredNotifications: existing}
			applyValuesFromUpdateCmd(maint, nil, &entity.UpdateMaintenanceCmd{
				DeferredNotifications: lo.ToPtr([]*entity.DeferredNotificationInput{}),
			})

			require.Empty(t, maint.DeferredNotifications)
		})

		t.Run("nil keeps the existing collection", func(t *testing.T) {
			t.Parallel()

			maint := &entity.Maintenance{DeferredNotifications: existing}
			applyValuesFromUpdateCmd(maint, nil, &entity.UpdateMaintenanceCmd{
				DeferredNotifications: nil,
			})

			require.Len(t, maint.DeferredNotifications, 1)
		})
	})

	t.Run("clearing is audited", func(t *testing.T) {
		t.Parallel()

		maint, _ := seedMaint(t)
		before := len(mocks.audit.all())

		err := service.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID:               maint.ID,
			DeferredNotifications: lo.ToPtr([]*entity.DeferredNotificationInput{}),
			Actor:                 actor(),
		})
		require.NoError(t, err)

		// Clearing hard-deletes rows, so the audit entry is the only trace.
		var cleared bool
		for _, action := range mocks.audit.all()[before:] {
			updated, ok := action.(audit.MaintUpdated)
			if !ok || updated.Maint.ID != maint.ID {
				continue
			}
			for _, change := range updated.Changes {
				if change.Field == "deferred_notifications" {
					cleared = true
				}
			}
		}
		require.True(t, cleared, "clearing reminders must record an audit change")
	})
}

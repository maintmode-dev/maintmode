package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestListOverdueNotStarted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)

	// makeMaint creates planned maints; start 20 min ago is well past any sane
	// grace window, start 1 min ago is fresh.
	overduePlanned := makeMaint(ctx, t, store, entity.NewPeriod(now.Add(-20*time.Minute), now.Add(-19*time.Minute)))
	fresh := makeMaint(ctx, t, store, entity.NewPeriod(now.Add(-time.Minute), now.Add(time.Hour)))

	// An overdue draft: never approved, but still "not started" → included.
	overdueDraft := makeMaintWithStatus(ctx, t, store,
		entity.NewPeriod(now.Add(-20*time.Minute), now.Add(-19*time.Minute)), entity.MaintenanceStatusDraft)

	// An overdue but already-running maint → excluded (it did start).
	overdueInProgress := makeMaintWithStatus(ctx, t, store,
		entity.NewPeriod(now.Add(-20*time.Minute), now.Add(-19*time.Minute)), entity.MaintenanceStatusInProgress)

	cutoff := now.Add(-15 * time.Minute)
	got, err := store.ListOverdueNotStarted(ctx, cutoff, 100_000)
	require.NoError(t, err)

	ids := lo.Map(got, func(m *entity.Maintenance, _ int) uuid.UUID { return m.ID })

	require.Contains(t, ids, overduePlanned.ID, "overdue planned maint must be returned")
	require.Contains(t, ids, overdueDraft.ID, "overdue draft maint must be returned")
	require.NotContains(t, ids, fresh.ID, "maint within grace window must be excluded")
	require.NotContains(t, ids, overdueInProgress.ID, "already-started maint must be excluded")

	// Every returned maint must be in a not-started status and started before the cutoff.
	for _, m := range got {
		require.Contains(t,
			[]entity.MaintenanceStatus{entity.MaintenanceStatusDraft, entity.MaintenanceStatusPlanned},
			m.Status)
		require.True(t, m.PlannedPeriod.Start.Before(cutoff),
			"returned maint %s started at %s, not before cutoff %s", m.ID, m.PlannedPeriod.Start, cutoff)
	}
}

// TestListOverdueNotStarted_OrderAndLimit checks oldest-first ordering and that
// the limit bounds the result set.
func TestListOverdueNotStarted_OrderAndLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	store := NewStore(db)

	older := makeMaint(ctx, t, store, entity.NewPeriod(now.Add(-40*time.Minute), now.Add(-39*time.Minute)))
	newer := makeMaint(ctx, t, store, entity.NewPeriod(now.Add(-30*time.Minute), now.Add(-29*time.Minute)))

	got, err := store.ListOverdueNotStarted(ctx, now.Add(-15*time.Minute), 100_000)
	require.NoError(t, err)

	idxOlder := lo.IndexOf(lo.Map(got, func(m *entity.Maintenance, _ int) uuid.UUID { return m.ID }), older.ID)
	idxNewer := lo.IndexOf(lo.Map(got, func(m *entity.Maintenance, _ int) uuid.UUID { return m.ID }), newer.ID)
	require.NotEqual(t, -1, idxOlder)
	require.NotEqual(t, -1, idxNewer)
	require.Less(t, idxOlder, idxNewer, "older planned start must come first (ascending order)")

	// limit is honored.
	limited, err := store.ListOverdueNotStarted(ctx, now.Add(-15*time.Minute), 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}

func makeMaintWithStatus(
	ctx context.Context, t *testing.T, store *Store, period entity.Period, status entity.MaintenanceStatus,
) *entity.Maintenance {
	t.Helper()

	created, err := store.CreateMaint(ctx, &entity.Maintenance{
		Title:          "Title" + t.Name(),
		Description:    "Description" + t.Name(),
		PlannedPeriod:  period,
		Scope:          entity.MaintenanceScopeResources,
		Status:         status,
		Impact:         entity.MaintenanceImpactFull,
		ApproverUserID: uuid.New(),
	})
	require.NoError(t, err)

	return created
}

package testdbutils

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

type MaintChanger func(m *entity.Maintenance)

func WithScope(scope entity.MaintenanceScope) MaintChanger {
	return func(m *entity.Maintenance) {
		m.Scope = scope
		if scope == entity.MaintenanceScopeGlobal {
			m.Resources = nil
		}
	}
}

func WithResources(ids ...uuid.UUID) MaintChanger {
	return func(m *entity.Maintenance) {
		m.Resources = ids
	}
}

func WithStatus(status entity.MaintenanceStatus) MaintChanger {
	return func(m *entity.Maintenance) {
		m.Status = status
	}
}

// WithActualPeriod pins the actual period regardless of status, e.g. to model
// drifted rows in the shared DB where a not-started maintenance carries an
// actual period.
func WithActualPeriod(period entity.Period) MaintChanger {
	return func(m *entity.Maintenance) {
		m.ActualPeriod = &period
	}
}

func WithSteps(step []*entity.MaintenanceStep) MaintChanger {
	return func(m *entity.Maintenance) {
		m.Steps = step
	}
}

// WithApprover pins the maintenance's approver to a specific user id (typically a
// real, seeded eligible approver) instead of the random default.
func WithApprover(approverID uuid.UUID) MaintChanger {
	return func(m *entity.Maintenance) {
		m.ApproverUserID = approverID
	}
}

func MakeResource(ctx context.Context, t *testing.T, store *resources.Store) *entity.ResourceDetails {
	t.Helper()

	resource, err := store.Create(ctx, &entity.ResourceDetails{
		Name:        "Resource" + xuuid.NewString(),
		Description: "Description" + t.Name(),
	})
	require.NoError(t, err)

	return resource
}

func MakeResources(ctx context.Context, t *testing.T, store *resources.Store, count int) []uuid.UUID {
	t.Helper()

	result := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, MakeResource(ctx, t, store).ID)
	}

	return result
}

func MakeMaint(
	ctx context.Context,
	t *testing.T,
	maintStore *maintenances.Store,
	resourceStore *resources.Store,
	period entity.Period,
	changers ...MaintChanger,
) *entity.Maintenance {
	t.Helper()

	maint := &entity.Maintenance{
		ID:             xuuid.New(),
		Title:          "Title" + t.Name(),
		Description:    "Description" + t.Name(),
		PlannedPeriod:  period,
		Scope:          entity.MaintenanceScopeResources,
		Status:         entity.MaintenanceStatusDraft,
		Impact:         entity.MaintenanceImpactFull,
		CreatedAt:      xtime.UTCNow(),
		ApproverUserID: xuuid.New(),
		Resources:      MakeResources(ctx, t, resourceStore, 2),
		Steps: []*entity.MaintenanceStep{{
			Order:               1,
			Description:         "Step 1" + t.Name(),
			RollbackDescription: "Rollback Step 1" + t.Name(),
			DurationMinutes:     1,
			Status:              entity.MaintenanceStepStatusPlanned,
		}},
	}
	for _, changer := range changers {
		changer(maint)
	}

	created, err := maintStore.CreateMaint(ctx, maint)
	require.NoError(t, err)

	if len(maint.Resources) > 0 {
		err = maintStore.AddResources(ctx, created.ID, maint.Resources)
		require.NoError(t, err)
		created.Resources = maint.Resources
	}
	if len(maint.Steps) > 0 {
		steps, err := maintStore.AddSteps(ctx, created.ID, maint.Steps)
		require.NoError(t, err)
		created.Steps = steps
	}

	closeActualPeriodOnCleanup(ctx, t, maintStore, created.ID)

	return created
}

// closeActualPeriodOnCleanup bounds a fixture's actual period once its test is
// done, so the row stops matching every future window.
//
// An open actual period is `[start, ∞)`, and `&&` reports it as overlapping ANY
// range — including every isolated window a later test claims. Nothing deletes
// fixtures from the shared database, so each run that started a maintenance and
// never finished it left behind a row that conflicts with everything, forever.
// Thousands had accumulated before this was added.
//
// That is invisible until a query has a LIMIT: ActualConflictedMaints caps at
// ActualConflictsLimit and orders by overlap start, so once enough of these
// accumulated they filled the page and pushed out the neighbor a test had just
// created — TestConflictScopeMatrix failed on rows it never created, and only
// in its global-scope cases, since resource-scoped ones were filtered out by
// their fresh resource ids.
//
// Time-based isolation cannot fix this. An unbounded range overlaps every
// window by definition, however far out the window is placed.
//
// The row is CLOSED rather than deleted: a test may legitimately assert that
// its fixture still exists, and a bounded period is what the maintenance would
// have carried had it been completed. The state is read back from the database
// rather than taken from the fixture, because the period is usually opened by
// the service under test (see services/maint/start_maint.go) long after this
// helper returned.
//
// Deliberately NOT reported as a failure: cleanup runs after the test's own
// assertions, and a janitorial write that could not land must not turn a
// passing test red.
func closeActualPeriodOnCleanup(
	ctx context.Context,
	t *testing.T,
	maintStore *maintenances.Store,
	maintID uuid.UUID,
) {
	t.Helper()

	t.Cleanup(func() {
		maint, err := maintStore.GetMaint(ctx, maintID)
		if err != nil || maint == nil || maint.ActualPeriod == nil || !maint.ActualPeriod.IsOpen() {
			return
		}

		// One hour, so the closed range stays inside the slot stride that
		// IsolatedPeriodBounds hands out and cannot reach a neighboring window.
		maint.ActualPeriod = lo.ToPtr(
			entity.NewPeriod(maint.ActualPeriod.Start, maint.ActualPeriod.Start.Add(time.Hour)),
		)

		_ = maintStore.UpdateMaint(ctx, maint)
	})
}

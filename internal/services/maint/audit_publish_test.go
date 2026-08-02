package maint

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// capturedAudit records the audit actions a service's mutations publish, so a
// test can assert which action (and actor) reached the publisher. It is wired as
// the default publisher stub in initService (see serviceMocks.audit).
type capturedAudit struct {
	mu      sync.Mutex
	actions []audit.Action
}

func (c *capturedAudit) record(_ context.Context, action audit.Action) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.actions = append(c.actions, action)
	return nil
}

func (c *capturedAudit) all() []audit.Action {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]audit.Action(nil), c.actions...)
}

func actor() *entity.User {
	return &entity.User{ID: uuid.New(), Email: "actor@example.com", Name: "Actor"}
}

// uniqueFutureOffset returns a large, effectively-unique positive time offset so
// a test can park its maintenance window where no other (parallel) test's window
// overlaps it. Derived from a random uuid rather than wall-clock so it stays
// distinct even across -count reruns within the same second.
func uniqueFutureOffset() time.Duration {
	b := uuid.New()
	// 16 bits of the uuid → up to ~65535 distinct hour offsets, all >= a decade
	// out so they never overlap the real-now windows other tests use.
	slot := time.Duration(uint16(b[0])<<8|uint16(b[1])) * time.Hour
	return 100000*time.Hour + slot
}

func TestAudit_PublishedPerMutation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Park this test's maintenance window far in the future, at a per-run-unique
	// offset. The approve subtest guards on a conflict fingerprint taken at
	// preview vs. approve; a parallel GLOBAL-scope maintenance (created by other
	// tests in this package) conflicts with ANY overlapping window regardless of
	// resources, so sharing the real-now window with them races the fingerprint.
	// A unique far-future window no parallel test overlaps removes the race.
	now := xtime.UTCNow().Add(uniqueFutureOffset())

	t.Run("start publishes MaintStarted with actor", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		recorder := mocks.audit
		a := actor()

		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		)

		require.NoError(t, service.StartMaint(ctx, &entity.StartMaintenanceCmd{MaintID: maint.ID, Actor: a}))

		got := lastAction[audit.MaintStarted](t, recorder)
		require.Equal(t, a.Email, got.Actor.Email)
		require.Equal(t, maint.ID, got.Maint.ID)
	})

	t.Run("create publishes MaintCreated with actor", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		mocks.expectAnyApproverEligible()
		recorder := mocks.audit
		a := actor()

		cmd := validCreateCmd(ctx, t, service, uuid.New())
		cmd.Actor = a

		maint, err := service.CreateDraft(ctx, cmd)
		require.NoError(t, err)

		got := lastAction[audit.MaintCreated](t, recorder)
		require.Equal(t, a.Email, got.Actor.Email)
		require.Equal(t, maint.ID, got.Maint.ID)
	})

	t.Run("approve publishes MaintApproved with actor", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		recorder := mocks.audit
		a := actor()

		// A fresh draft maint with no overlaps: the live conflict set is empty,
		// so the snapshot we feed matches and approve proceeds.
		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
		)

		conflicts, err := service.conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs:   maint.Resources,
		})
		require.NoError(t, err)

		// Only the assigned approver may approve; act as that user.
		a.ID = maint.ApproverUserID
		require.NoError(t, service.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ActorUserID:           maint.ApproverUserID,
			Actor:                 a,
			ConflictSnapshot:      entity.ConflictsSnapshot{Conflicts: conflicts},
		}))

		got := lastAction[audit.MaintApproved](t, recorder)
		require.Equal(t, a.Email, got.Actor.Email)
		require.Equal(t, maint.ID, got.Maint.ID)
	})

	t.Run("admin override publishes MaintApproved with the admin as actor", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		recorder := mocks.audit

		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
		)

		conflicts, err := service.conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs:   maint.Resources,
		})
		require.NoError(t, err)

		// An admin approving someone else's maintenance: the audit must record
		// the admin who acted, not the user the maintenance was assigned to, so
		// the override is attributable.
		admin := actor()
		admin.Roles = []entity.Role{entity.RoleAdmin}
		require.NoError(t, service.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ActorUserID:           admin.ID,
			Actor:                 admin,
			ConflictSnapshot:      entity.ConflictsSnapshot{Conflicts: conflicts},
		}))

		got := lastAction[audit.MaintApproved](t, recorder)
		require.Equal(t, admin.Email, got.Actor.Email)
		require.Equal(t, admin.ID, got.Actor.ID)

		// The published action must actually render as an override, not merely
		// carry an actor that happens to differ from the assignment.
		payload, err := audit.NewRenderer().Render(got)
		require.NoError(t, err)
		require.Contains(t, payload.Details, "on behalf of the assigned approver")
	})

	t.Run("complete publishes MaintCompleted with actor", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		recorder := mocks.audit
		a := actor()

		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		)

		// planned -> in_progress, then drive every step to a terminal state so
		// the maint may complete.
		require.NoError(t, service.StartMaint(ctx, &entity.StartMaintenanceCmd{MaintID: maint.ID, Actor: a}))
		for _, step := range maint.Steps {
			require.NoError(t, service.StartStep(ctx, &entity.StartMaintenanceStepCmd{MaintID: maint.ID, StepID: step.ID, Actor: a}))
			require.NoError(t, service.CompleteStep(ctx, &entity.CompleteMaintenanceStepCmd{MaintID: maint.ID, StepID: step.ID, Actor: a}))
		}

		require.NoError(t, service.CompleteMaint(ctx, &entity.CompleteMaintenanceCmd{MaintID: maint.ID, Actor: a}))

		got := lastAction[audit.MaintCompleted](t, recorder)
		require.Equal(t, a.Email, got.Actor.Email)
		require.Equal(t, maint.ID, got.Maint.ID)
	})

	t.Run("StartStep publishes exactly one MaintStepStarted with actor", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		recorder := mocks.audit
		a := actor()

		maint := makeInProgressOneStepMaint(ctx, t, service, now, entity.MaintenanceStepStatusPlanned)

		require.NoError(t, service.StartStep(ctx, &entity.StartMaintenanceStepCmd{
			MaintID: maint.ID, StepID: maint.Steps[0].ID, Actor: a,
		}))

		got := lastAction[audit.MaintStepStarted](t, recorder)
		require.Equal(t, a.Email, got.Actor.Email)
		require.Equal(t, maint.ID, got.Maint.ID)
		require.Equal(t, maint.Steps[0].ID, got.Step.ID)
		// One step transition => exactly one step-started action (guards the loop).
		require.Equal(t, 1, countActions[audit.MaintStepStarted](recorder))
	})

	t.Run("CompleteStep publishes exactly one MaintStepCompleted with actor", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		recorder := mocks.audit
		a := actor()

		maint := makeInProgressOneStepMaint(ctx, t, service, now, entity.MaintenanceStepStatusStarted)

		require.NoError(t, service.CompleteStep(ctx, &entity.CompleteMaintenanceStepCmd{
			MaintID: maint.ID, StepID: maint.Steps[0].ID, Actor: a,
		}))

		got := lastAction[audit.MaintStepCompleted](t, recorder)
		require.Equal(t, a.Email, got.Actor.Email)
		require.Equal(t, maint.ID, got.Maint.ID)
		require.Equal(t, maint.Steps[0].ID, got.Step.ID)
		require.Equal(t, 1, countActions[audit.MaintStepCompleted](recorder))
	})

	t.Run("CancelStep publishes exactly one MaintStepCancelled with actor", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		recorder := mocks.audit
		a := actor()

		maint := makeInProgressOneStepMaint(ctx, t, service, now, entity.MaintenanceStepStatusStarted)

		require.NoError(t, service.CancelStep(ctx, &entity.CancelMaintenanceStepCmd{
			MaintID: maint.ID, StepID: maint.Steps[0].ID, Actor: a,
		}))

		got := lastAction[audit.MaintStepCancelled](t, recorder)
		require.Equal(t, a.Email, got.Actor.Email)
		require.Equal(t, maint.ID, got.Maint.ID)
		require.Equal(t, maint.Steps[0].ID, got.Step.ID)
		require.Equal(t, 1, countActions[audit.MaintStepCancelled](recorder))
	})

	t.Run("cancel publishes MaintCancelled with actor", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		recorder := mocks.audit
		a := actor()

		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		)

		require.NoError(t, service.CancelMaint(ctx, &entity.CancelMaintenanceCmd{
			MaintID: maint.ID,
			Reason:  entity.MaintenanceCancelReasonMistake,
			Actor:   a,
		}))

		got := lastAction[audit.MaintCancelled](t, recorder)
		require.Equal(t, a.Email, got.Actor.Email)
		require.Equal(t, entity.MaintenanceCancelReasonMistake, got.Maint.CancelReason)
	})

	t.Run("idempotent no-op cancel does not publish", func(t *testing.T) {
		t.Parallel()
		service, mocks := initService(t)
		recorder := mocks.audit

		maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
			entity.NewPeriod(now, now.Add(time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusCancelled),
		)

		require.NoError(t, service.CancelMaint(ctx, &entity.CancelMaintenanceCmd{
			MaintID: maint.ID,
			Reason:  entity.MaintenanceCancelReasonMistake,
			Actor:   actor(),
		}))

		require.Empty(t, recorder.all(), "an already-canceled maint is a no-op: no audit row")
	})
}

func TestMaintUpdateChanges(t *testing.T) {
	t.Parallel()

	base := func() *entity.Maintenance {
		return &entity.Maintenance{
			ID:             uuid.New(),
			Title:          "T",
			Description:    "D",
			PlannedPeriod:  entity.NewPeriod(xtime.UTCNow(), xtime.UTCNow().Add(time.Hour)),
			Scope:          entity.MaintenanceScopeGlobal,
			Impact:         entity.MaintenanceImpactNone,
			ApproverUserID: uuid.New(),
		}
	}

	t.Run("identical scalars + cmd supplying no collection yields no changes", func(t *testing.T) {
		t.Parallel()
		prev := base()
		current := *prev // value copy: same scalars
		require.Empty(t, maintUpdateChanges(prev, &current, &entity.UpdateMaintenanceCmd{}))
	})

	t.Run("title change recorded before/after", func(t *testing.T) {
		t.Parallel()
		prev := base()
		current := *prev
		current.Title = "T2"

		changes := maintUpdateChanges(prev, &current, &entity.UpdateMaintenanceCmd{})
		require.Len(t, changes, 1)
		require.Equal(t, "title", changes[0].Field)
		require.Equal(t, "T", changes[0].Old)
		require.Equal(t, "T2", changes[0].New)
	})

	t.Run("multiple scalar changes recorded", func(t *testing.T) {
		t.Parallel()
		prev := base()
		current := *prev
		current.Scope = entity.MaintenanceScopeResources
		current.Impact = entity.MaintenanceImpactFull

		changes := maintUpdateChanges(prev, &current, &entity.UpdateMaintenanceCmd{})
		fields := lo.Map(changes, func(c entity.AuditFieldChange, _ int) string { return c.Field })
		require.ElementsMatch(t, []string{"scope", "impact"}, fields)
	})

	t.Run("steps supplied by cmd recorded as changed flag", func(t *testing.T) {
		t.Parallel()
		prev := base()
		current := *prev

		// The collection flag mirrors the update's replace rule: it fires when the
		// command supplied a replacement set, not on a prev↔current content diff.
		cmd := &entity.UpdateMaintenanceCmd{
			Steps: []*entity.MaintenanceStepInput{{Order: 1, Description: "s", DurationMinutes: 5}},
		}
		changes := maintUpdateChanges(prev, &current, cmd)
		require.Len(t, changes, 1)
		require.Equal(t, "steps", changes[0].Field)
		require.Empty(t, changes[0].Old)
		require.Empty(t, changes[0].New)
	})

	t.Run("notify_targets supplied by cmd recorded as changed flag", func(t *testing.T) {
		t.Parallel()
		prev := base()
		current := *prev

		cmd := &entity.UpdateMaintenanceCmd{
			NotifyTargets: []*entity.NotifyTargetInput{{ChannelID: uuid.New()}},
		}
		changes := maintUpdateChanges(prev, &current, cmd)
		require.Len(t, changes, 1)
		require.Equal(t, "notify_targets", changes[0].Field)
	})
}

// makeInProgressOneStepMaint seeds an in_progress maintenance with a single step
// in stepStatus, so a step transition can be driven without first walking the
// whole maint lifecycle. The maint period is in the future so the overdue sweep
// (shared DB) does not touch it.
func makeInProgressOneStepMaint(
	ctx context.Context,
	t *testing.T,
	service *Service,
	now time.Time,
	stepStatus entity.MaintenanceStepStatus,
) *entity.Maintenance {
	t.Helper()
	return testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore,
		entity.NewPeriod(now, now.Add(time.Hour)),
		testdbutils.WithStatus(entity.MaintenanceStatusInProgress),
		testdbutils.WithSteps([]*entity.MaintenanceStep{{
			Order:               1,
			Description:         "step1" + t.Name(),
			RollbackDescription: "rollback step1" + t.Name(),
			Status:              stepStatus,
		}}),
	)
}

// countActions returns how many captured actions are of type T.
func countActions[T audit.Action](recorder *capturedAudit) int {
	var n int
	for _, a := range recorder.all() {
		if _, ok := a.(T); ok {
			n++
		}
	}
	return n
}

// lastAction returns the most recent captured action of type T, failing if none.
func lastAction[T audit.Action](t *testing.T, recorder *capturedAudit) T {
	t.Helper()
	actions := recorder.all()
	for i := len(actions) - 1; i >= 0; i-- {
		if a, ok := actions[i].(T); ok {
			return a
		}
	}
	var zero T
	require.Failf(t, "no captured action of expected type", "%T not found in %d actions", zero, len(actions))
	return zero
}

// findAuditCancel returns the MaintCancelled action published for the given
// maintenance id. The auto-cancel sweep runs on the shared DB and may cancel
// other tests' overdue maints too, so match by id rather than taking the last.
func findAuditCancel(t *testing.T, recorder *capturedAudit, maintID uuid.UUID) audit.MaintCancelled {
	t.Helper()
	for _, a := range recorder.all() {
		if mc, ok := a.(audit.MaintCancelled); ok && mc.Maint.ID == maintID {
			return mc
		}
	}
	require.Failf(t, "no MaintCancelled for maint", "maint %s not in captured actions", maintID)
	return audit.MaintCancelled{}
}

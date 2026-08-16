package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// The status matrix, end to end. Each case pins which of the three conflict
// sources a card reads from, because that choice is the whole feature: get it
// wrong for in_progress and a live card silently starts answering a different
// question.
func TestGetConflictsByLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := initService(db)

	// The neighbor every case is probed with: it RAN over the window and was
	// then canceled, so the live query rejects it on status while the factual
	// query must report it. One fixture, three different answers.
	seed := func(t *testing.T, subjectChangers ...testdbutils.MaintChanger) (
		subject *entity.Maintenance, neighbor *entity.Maintenance, period entity.Period,
	) {
		t.Helper()
		start, end := testdbutils.IsolatedPeriodBounds(t)
		period = entity.NewPeriod(start, end)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		subject = testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore, period,
			append([]testdbutils.MaintChanger{
				testdbutils.WithScope(entity.MaintenanceScopeResources),
				testdbutils.WithResources(shared.ID),
			}, subjectChangers...)...,
		)
		neighborPeriod := entity.NewPeriod(start.Add(time.Hour), end)
		neighbor = testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore, neighborPeriod,
			testdbutils.WithStatus(entity.MaintenanceStatusCancelled),
			testdbutils.WithActualPeriod(neighborPeriod),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		return subject, neighbor, period
	}

	cmdFor := func(maint *entity.Maintenance) *calendardto.ConflictQueryCmd {
		return &calendardto.ConflictQueryCmd{
			MaintID:       maint.ID,
			Status:        maint.Status,
			PlannedPeriod: maint.PlannedPeriod,
			ActualPeriod:  maint.ActualPeriod,
			Scope:         maint.Scope,
			ResourceIDs:   maint.Resources,
		}
	}

	reports := func(conflicts []*calendardto.Conflict, id uuid.UUID) bool {
		return lo.ContainsBy(conflicts, func(c *calendardto.Conflict) bool {
			return c.MaintenanceID == id
		})
	}

	// seedNeverRan builds a canceled subject with NO actual period, alongside a
	// planned neighbor over the same window. That neighbor conflicts on every
	// axis, so whether it surfaces turns purely on the approval snapshot.
	seedNeverRan := func(t *testing.T) (
		subject *entity.Maintenance, foreseen *entity.Maintenance,
		period entity.Period, sharedID uuid.UUID,
	) {
		t.Helper()
		start, end := testdbutils.IsolatedPeriodBounds(t)
		period = entity.NewPeriod(start, end)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		subject = testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore, period,
			testdbutils.WithStatus(entity.MaintenanceStatusCancelled),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		foreseen = testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore, period,
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		return subject, foreseen, period, shared.ID
	}

	// approveWith freezes an approval snapshot naming the given neighbors over
	// the whole window — the record the snapshot branch reads back.
	approveWith := func(
		t *testing.T,
		subjectID uuid.UUID,
		period entity.Period,
		sharedID uuid.UUID,
		neighbors ...*entity.Maintenance,
	) {
		t.Helper()
		require.NoError(t, snapshotStore.Save(ctx, subjectID,
			lo.Map(neighbors, func(n *entity.Maintenance, _ int) *entity.ConflictWithResources {
				return &entity.ConflictWithResources{
					Conflict: &entity.Conflict{
						MaintenanceID: n.ID,
						Title:         n.Title,
						Scope:         n.Scope,
						OverlapStart:  period.Start,
						OverlapEnd:    lo.FromPtr(period.End),
					},
					Resources: []uuid.UUID{sharedID},
				}
			}),
		))
	}

	// AC14: the live statuses keep reading the live query, which filters
	// neighbors by status — so the canceled neighbor stays invisible.
	for _, status := range []entity.MaintenanceStatus{
		entity.MaintenanceStatusDraft,
		entity.MaintenanceStatusPlanned,
	} {
		t.Run("live path for "+string(status), func(t *testing.T) {
			t.Parallel()

			subject, neighbor, _ := seed(t, testdbutils.WithStatus(status))

			conflicts, err := s.GetConflicts(ctx, cmdFor(subject))
			require.NoError(t, err)
			require.False(t, reports(conflicts, neighbor.ID),
				"a live card compares plans, so a canceled neighbor must stay filtered out")
		})
	}

	// The in_progress case is the one a single-key branch gets wrong: its actual
	// period is non-nil (open), so keying on the period alone would route it to
	// the factual path.
	t.Run("in_progress keeps the live path despite having an actual period", func(t *testing.T) {
		t.Parallel()

		subject, neighbor, period := seed(t,
			testdbutils.WithStatus(entity.MaintenanceStatusInProgress),
		)
		// An open actual period, as a started maintenance really carries.
		subject.ActualPeriod = lo.ToPtr(entity.NewOpenEndedPeriod(period.Start))

		conflicts, err := s.GetConflicts(ctx, cmdFor(subject))
		require.NoError(t, err)
		require.False(t, reports(conflicts, neighbor.ID),
			"in_progress answers 'may I work now', which is a question about plans")
	})

	// AC5: the headline case. The neighbor ran alongside and was never in this
	// maintenance's approval snapshot, so it surfaces unflagged.
	t.Run("completed reads factual overlaps and flags unreviewed work", func(t *testing.T) {
		t.Parallel()

		subject, neighbor, period := seed(t,
			testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
		)
		// The factual query matches on the window carried in the cmd, so the
		// subject only needs it here — same idiom as the in_progress case above.
		subject.ActualPeriod = lo.ToPtr(period)

		conflicts, err := s.GetConflicts(ctx, cmdFor(subject))
		require.NoError(t, err)

		actual, found := lo.Find(conflicts, func(c *calendardto.Conflict) bool {
			return c.MaintenanceID == neighbor.ID
		})
		require.True(t, found,
			"work that ran alongside must surface even though it was canceled — "+
				"this is the maintenance the live query could never show")
		require.False(t, actual.KnownAtApproval,
			"nobody reviewed this neighbor, which is what an incident review looks for")
		require.NotEmpty(t, actual.Resources, "the conflict names what the neighbor touched")
	})

	// AC5's other half: unreviewed conflicts lead. With a single neighbor in the
	// fixture, "flagged correctly" and "not flagged at all" are indistinguishable
	// — dropping MarkApprovalState from the factual branch would pass. Two
	// neighbors, one in the approval snapshot and one not, pin both the flag and
	// the order the reviewer actually reads.
	t.Run("unreviewed conflicts sort ahead of reviewed ones", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		period := entity.NewPeriod(start, end)
		shared := testdbutils.MakeResource(ctx, t, resourcesStore)

		subject := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore, period,
			testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
			testdbutils.WithActualPeriod(period),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)

		// Starts FIRST, so overlap start alone would place it ahead — only the
		// flag can demote it.
		reviewedPeriod := entity.NewPeriod(start.Add(time.Hour), end)
		reviewed := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore, reviewedPeriod,
			testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
			testdbutils.WithActualPeriod(reviewedPeriod),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)
		unreviewedPeriod := entity.NewPeriod(start.Add(2*time.Hour), end)
		unreviewed := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore, unreviewedPeriod,
			testdbutils.WithStatus(entity.MaintenanceStatusCancelled),
			testdbutils.WithActualPeriod(unreviewedPeriod),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(shared.ID),
		)

		approveWith(t, subject.ID, period, shared.ID, reviewed)

		conflicts, err := s.GetConflicts(ctx, cmdFor(subject))
		require.NoError(t, err)

		ordered := lo.Filter(conflicts, func(c *calendardto.Conflict, _ int) bool {
			return c.MaintenanceID == reviewed.ID || c.MaintenanceID == unreviewed.ID
		})
		require.Len(t, ordered, 2)
		require.Equal(t, unreviewed.ID, ordered[0].MaintenanceID,
			"work nobody reviewed leads, even though it started later")
		require.False(t, ordered[0].KnownAtApproval)
		require.True(t, ordered[1].KnownAtApproval,
			"the neighbor named in the approval snapshot is flagged as seen")
	})

	// AC7: no factual window, so the card falls back to what was foreseen.
	t.Run("canceled before start reads the approval snapshot", func(t *testing.T) {
		t.Parallel()

		subject, foreseen, period, sharedID := seedNeverRan(t)

		approveWith(t, subject.ID, period, sharedID, foreseen)

		conflicts, err := s.GetConflicts(ctx, cmdFor(subject))
		require.NoError(t, err)

		actual, found := lo.Find(conflicts, func(c *calendardto.Conflict) bool {
			return c.MaintenanceID == foreseen.ID
		})
		require.True(t, found, "a maintenance that never ran still shows what was known at approval")
		require.True(t, actual.KnownAtApproval,
			"everything on the snapshot branch was seen by the approver by construction")
	})

	// AC8.
	t.Run("canceled from draft reports nothing", func(t *testing.T) {
		t.Parallel()

		// Same fixture as above, minus the approval: the planned neighbor would
		// conflict on every axis, but our subject was canceled before it was ever
		// approved, so no snapshot exists to report it from.
		subject, _, _, _ := seedNeverRan(t)

		conflicts, err := s.GetConflicts(ctx, cmdFor(subject))
		require.NoError(t, err)
		require.Empty(t, conflicts, "no approval ever happened, so there is nothing to report")
	})

	// A terminal maintenance whose period was nulled by a drifted clock must
	// degrade to the snapshot rather than reach the factual query's precondition.
	t.Run("terminal with an open actual period degrades to the snapshot", func(t *testing.T) {
		t.Parallel()

		start, end := testdbutils.IsolatedPeriodBounds(t)
		subject := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithStatus(entity.MaintenanceStatusCompleted),
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
		)

		cmd := cmdFor(subject)
		cmd.ActualPeriod = lo.ToPtr(entity.NewOpenEndedPeriod(start))

		conflicts, err := s.GetConflicts(ctx, cmd)
		require.NoError(t, err, "an anomalous row must not fail the card")
		require.Empty(t, conflicts)
	})
}

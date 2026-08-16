package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// TestConflictScopeMatrix walks every scope pairing in one table, asserting both
// halves of the answer together: whether the two maintenances conflict at all
// (ConflictedMaints), and which resources the conflict then reports
// (ConflictedResources).
//
// Testing them as a pair is the point. Detection and resource reporting are two
// queries with two different predicates, and the bug this suite grew out of was
// precisely a disagreement between them — a pair that conflicted by scope while
// the resource half returned nothing.
//
// The asymmetric rows matter most. A subset relation in one direction is not the
// same fixture as the other direction, and an EXISTS subquery joined the wrong
// way round passes one while failing the other; the pre-existing cases were all
// symmetric and could not tell them apart.
func TestConflictScopeMatrix(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := conflicts.NewStore(db)

	resourceA := testdbutils.MakeResource(ctx, t, resourcesStore)
	resourceB := testdbutils.MakeResource(ctx, t, resourcesStore)

	for _, tc := range []struct {
		name string
		// subject/neighbor resources; nil means global scope
		subjectResources  []uuid.UUID
		neighborResources []uuid.UUID
		// forceSubjectScope overrides the scope that would follow from the
		// resource list, for the one row that needs a resource-scoped
		// maintenance holding no resources at all.
		forceSubjectScope entity.MaintenanceScope
		wantConflict      bool
		// wantReported is what the conflict's resources must list: the NEIGHBOR's
		// own set, never the intersection. Empty for a global-scope neighbor,
		// which owns no resources.
		wantReported []uuid.UUID
	}{
		{
			name:         "global x global — conflict by scope, no resources anywhere",
			wantConflict: true,
			wantReported: nil,
		},
		{
			name:              "global x resource — conflict by our global scope",
			neighborResources: []uuid.UUID{resourceA.ID},
			wantConflict:      true,
			wantReported:      []uuid.UUID{resourceA.ID},
		},
		{
			name:             "resource x global — conflict by the neighbor's global scope",
			subjectResources: []uuid.UUID{resourceA.ID},
			wantConflict:     true,
			wantReported:     nil,
		},
		{
			name:              "resource[A] x resource[A,B] — we are the subset",
			subjectResources:  []uuid.UUID{resourceA.ID},
			neighborResources: []uuid.UUID{resourceA.ID, resourceB.ID},
			wantConflict:      true,
			// B is reported even though we do not hold it: the field answers what
			// the neighbor touches.
			wantReported: []uuid.UUID{resourceA.ID, resourceB.ID},
		},
		{
			name:              "resource[A,B] x resource[A] — the neighbor is the subset",
			subjectResources:  []uuid.UUID{resourceA.ID, resourceB.ID},
			neighborResources: []uuid.UUID{resourceA.ID},
			wantConflict:      true,
			wantReported:      []uuid.UUID{resourceA.ID},
		},
		{
			name:              "resource[A,B] x resource[A,B] — overlap on both",
			subjectResources:  []uuid.UUID{resourceA.ID, resourceB.ID},
			neighborResources: []uuid.UUID{resourceA.ID, resourceB.ID},
			wantConflict:      true,
			wantReported:      []uuid.UUID{resourceA.ID, resourceB.ID},
		},
		{
			name:              "resource[A] x resource[B] — disjoint, no conflict",
			subjectResources:  []uuid.UUID{resourceA.ID},
			neighborResources: []uuid.UUID{resourceB.ID},
			wantConflict:      false,
		},
		{
			// A resource-scoped maintenance holding nothing matches global
			// neighbors only: the resource branch is built solely when the id
			// list is non-empty and defaults to false otherwise. That default is
			// the one line of the predicate no other row reaches, and both
			// queries carry their own copy of it.
			name:              "resource[] x resource[A] — an empty set shares nothing",
			subjectResources:  nil,
			forceSubjectScope: entity.MaintenanceScopeResources,
			neighborResources: []uuid.UUID{resourceA.ID},
			wantConflict:      false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A private window per row: a global-scope fixture conflicts with
			// every overlapping maintenance regardless of resources, so rows
			// sharing a window would see each other.
			start, end := testdbutils.IsolatedPeriodBounds(t)

			subject := makeScopedMaint(ctx, t, start, end, tc.subjectResources,
				entity.MaintenanceStatusDraft, tc.forceSubjectScope)
			neighbor := makeScopedMaint(ctx, t, start, end, tc.neighborResources,
				entity.MaintenanceStatusPlanned, "")

			conflicted, err := store.ConflictedMaints(ctx, &entity.ConflictQueryCmd{
				MaintID:       subject.ID,
				Scope:         subject.Scope,
				PlannedPeriod: subject.PlannedPeriod,
				ResourceIDs:   subject.Resources,
			})
			require.NoError(t, err)

			found := lo.ContainsBy(conflicted, func(c *entity.Conflict) bool {
				return c.MaintenanceID == neighbor.ID
			})

			if !tc.wantConflict {
				require.False(t, found, "these maintenances must not conflict")
				return
			}
			require.True(t, found, "these maintenances must conflict")

			resources, err := store.ConflictedResources(ctx, &entity.ConflictResourcesQueryCmd{
				ConflictedMaintIDs: []uuid.UUID{neighbor.ID},
			})
			require.NoError(t, err)

			// Compared as a set: the query has no ORDER BY.
			require.ElementsMatch(t, tc.wantReported, resources[neighbor.ID],
				"a conflict reports the neighbor's own resources")
		})

		// The same pairing against the factual query. It carries its own copy of
		// the scope predicate — deliberately, so the approve gate's query stays
		// untouched — which means it inherits no coverage from the rows above and
		// needs its own. Without this, flipping the copy's resource branch to
		// always-true passes every other test in the suite while leaking
		// unrelated maintenances into an incident review.
		t.Run(tc.name+" (factual)", func(t *testing.T) {
			t.Parallel()

			start, end := testdbutils.IsolatedPeriodBounds(t)
			period := entity.NewPeriod(start, end)

			// Both sides must have actually run for the factual query to see
			// them, so the statuses differ from the live rows above.
			subject := makeScopedMaint(ctx, t, start, end, tc.subjectResources,
				entity.MaintenanceStatusCompleted, tc.forceSubjectScope,
				testdbutils.WithActualPeriod(period))
			neighbor := makeScopedMaint(ctx, t, start, end, tc.neighborResources,
				entity.MaintenanceStatusCancelled, "", testdbutils.WithActualPeriod(period))

			conflicted, _, err := store.ActualConflictedMaints(ctx, &entity.ActualConflictQueryCmd{
				MaintID:      subject.ID,
				Scope:        subject.Scope,
				ActualPeriod: period,
				ResourceIDs:  subject.Resources,
			})
			require.NoError(t, err)

			found := lo.ContainsBy(conflicted, func(c *entity.Conflict) bool {
				return c.MaintenanceID == neighbor.ID
			})
			require.Equal(t, tc.wantConflict, found,
				"the factual query must pair scopes exactly as the live one does")
		})
	}
}

// makeScopedMaint builds a maintenance whose scope follows from its resources:
// none means global. Status is a parameter because only planned and in-progress
// maintenances are counted as conflicts, so a neighbor has to be planned while
// the subject can stay a draft.
//
// forceScope, when non-empty, replaces the derived scope. It exists for the one
// combination the derivation cannot express: resource scope with an empty
// resource set.
func makeScopedMaint(
	ctx context.Context,
	t *testing.T,
	start, end time.Time,
	resources []uuid.UUID,
	status entity.MaintenanceStatus,
	forceScope entity.MaintenanceScope,
	extra ...testdbutils.MaintChanger,
) *entity.Maintenance {
	t.Helper()

	scope := entity.MaintenanceScopeGlobal
	if len(resources) > 0 {
		scope = entity.MaintenanceScopeResources
	}

	opts := []testdbutils.MaintChanger{
		testdbutils.WithStatus(status),
		testdbutils.WithScope(scope),
	}
	if len(resources) > 0 {
		opts = append(opts, testdbutils.WithResources(resources...))
	}
	if forceScope != "" {
		opts = append(opts, testdbutils.WithScope(forceScope))
	}

	return testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
		entity.NewPeriod(start, end), append(opts, extra...)...)
}

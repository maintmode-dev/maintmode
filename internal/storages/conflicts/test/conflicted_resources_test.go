package test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
)

// What this method returns for each scope pairing is covered by
// TestConflictScopeMatrix, which asserts it alongside the detection half rather
// than in isolation. Left here is what the matrix cannot reach: the guard in
// front of the query.
func TestConflictedResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store := conflicts.NewStore(db)

	// This store method does not decide WHICH maintenances conflict — that is
	// ConflictedMaints' job, and it hands the ids down. So the only way to get an
	// empty result is to ask about nothing.
	//
	// Both spellings of "nothing" are tested because they reach the guard
	// differently and only one of them is obviously safe: a nil slice is the
	// natural zero value, while an allocated-but-empty slice is what a caller
	// building ids from a filtered list produces.
	for _, tc := range []struct {
		name string
		ids  []uuid.UUID
	}{
		{name: "nil id slice", ids: nil},
		{name: "empty id slice", ids: []uuid.UUID{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// The guard is load-bearing, not a fast path: without it an empty
			// slice reaches postgres.ARRAY and serializes to an untyped `ARRAY[]`
			// whose element type Postgres cannot infer. GetConflicts calls this on
			// every read, so a maintenance with no conflicts — the common case —
			// would fail the whole request.
			resources, err := store.ConflictedResources(ctx, &entity.ConflictResourcesQueryCmd{
				ConflictedMaintIDs: tc.ids,
			})
			require.NoError(t, err, "an empty id list must not reach the query")
			require.Empty(t, resources)
		})
	}
}

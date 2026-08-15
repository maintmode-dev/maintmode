package entity

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	resID1 = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	resID2 = uuid.MustParse("20000000-0000-0000-0000-000000000002")
	resID3 = uuid.MustParse("30000000-0000-0000-0000-000000000003")
)

// The empty set is the one input with a hardcoded answer rather than a computed
// one, so it gets its own check. Everything else about the fingerprint —
// determinism, what changes it, order-independence, time normalization — is
// covered by the tests below, which assert those properties directly instead of
// asserting that a hash looks like a hash.
func TestConflictFingerprint_EmptySet(t *testing.T) {
	empty := ConflictFingerprint(nil)

	require.Len(t, empty, 64)
	require.Equal(t, empty, ConflictFingerprint([]*ConflictWithResources{}),
		"nil and an allocated empty slice mean the same thing")
	require.NotEqual(t, empty, ConflictFingerprint([]*ConflictWithResources{
		{
			Conflict: &Conflict{
				MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Scope:         MaintenanceScopeGlobal,
			},
		},
	}), "an empty set must not collide with a populated one")
}

func TestConflictFingerprint_Deterministic(t *testing.T) {
	conflicts := []*ConflictWithResources{
		{
			Conflict: &Conflict{
				MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				Title:         "Test Conflict 1",
				OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Scope:         MaintenanceScopeResources,
			},
			Resources: []uuid.UUID{resID1, resID2},
		},
	}

	fingerprints := make([]string, 10)
	for i := range 10 {
		fingerprints[i] = ConflictFingerprint(conflicts)
	}

	for i := 1; i < len(fingerprints); i++ {
		require.Equal(t, fingerprints[0], fingerprints[i])
	}
}

func TestConflictFingerprint_DifferentInputs(t *testing.T) {
	testCases := []struct {
		name      string
		conflicts []*ConflictWithResources
	}{
		{
			name: "different maintenance ID",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeGlobal,
					},
					Resources: nil,
				},
			},
		},
		{
			name: "different scope",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: nil,
				},
			},
		},
		{
			name: "different resources",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: []uuid.UUID{resID1},
				},
			},
		},
	}

	fingerprints := make(map[string]bool)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fp := ConflictFingerprint(tc.conflicts)
			require.NotContains(t, fingerprints, fp)
			fingerprints[fp] = true
		})
	}
}

func TestConflictFingerprint_OrderIndependence(t *testing.T) {
	makeBase := func() []*ConflictWithResources {
		return []*ConflictWithResources{
			{
				Conflict: &Conflict{
					MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
					Title:         "Test Conflict 1",
					OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
					OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
					Scope:         MaintenanceScopeResources,
				},
				Resources: []uuid.UUID{resID1, resID2},
			},
			{
				Conflict: &Conflict{
					MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
					Title:         "Test Conflict 2",
					OverlapStart:  time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
					OverlapEnd:    time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
					Scope:         MaintenanceScopeGlobal,
				},
				Resources: nil,
			},
		}
	}

	baseConflicts := makeBase()
	reversedConflicts := makeBase()
	slices.Reverse(reversedConflicts)
	for _, c := range reversedConflicts {
		slices.Reverse(c.Resources)
	}

	fp1 := ConflictFingerprint(baseConflicts)
	fp2 := ConflictFingerprint(reversedConflicts)

	require.Equal(t, fp1, fp2)
}

func TestConflictFingerprint_TimeNormalization(t *testing.T) {
	// Same UTC instant expressed differently must produce the same fingerprint
	// (timezone offset is stripped via UTC(), sub-second precision via Truncate(time.Second)).
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	loc := time.FixedZone("UTC+3", 3*3600)
	sameInstantInOtherTZ := baseTime.In(loc)

	conflictAt := func(start time.Time) []*ConflictWithResources {
		return []*ConflictWithResources{
			{
				Conflict: &Conflict{
					MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
					Title:         "Test Conflict 1",
					OverlapStart:  start,
					OverlapEnd:    start.Add(2 * time.Hour),
					Scope:         MaintenanceScopeGlobal,
				},
				Resources: nil,
			},
		}
	}

	fpUTC := ConflictFingerprint(conflictAt(baseTime))
	fpOtherTZ := ConflictFingerprint(conflictAt(sameInstantInOtherTZ))
	require.Equal(t, fpUTC, fpOtherTZ, "fingerprint should be timezone-independent")

	for _, sub := range []time.Duration{500 * time.Millisecond, 999 * time.Millisecond, 123456789 * time.Nanosecond} {
		fp := ConflictFingerprint(conflictAt(baseTime.Add(sub)))
		require.Equal(t, fpUTC, fp, "fingerprint should truncate sub-second precision (offset=%s)", sub)
	}
}

func TestSortResources(t *testing.T) {
	tests := []struct {
		name      string
		resources []uuid.UUID
		want      []uuid.UUID
	}{
		{
			name:      "empty slice",
			resources: []uuid.UUID{},
			want:      []uuid.UUID{},
		},
		{
			name:      "nil slice",
			resources: nil,
			want:      nil,
		},
		{
			name:      "already sorted",
			resources: []uuid.UUID{resID1, resID2},
			want:      []uuid.UUID{resID1, resID2},
		},
		{
			name:      "reverse order",
			resources: []uuid.UUID{resID2, resID1},
			want:      []uuid.UUID{resID1, resID2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortResources(tt.resources)
			require.Equal(t, tt.want, tt.resources)
		})
	}
}

func TestMarkKnownAtApproval(t *testing.T) {
	maintA := uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	maintB := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000002")

	newConflict := func(id uuid.UUID, start time.Time, resources ...uuid.UUID) *ConflictWithResources {
		return &ConflictWithResources{
			Conflict: &Conflict{
				MaintenanceID: id,
				Title:         "conflict",
				OverlapStart:  start,
				OverlapEnd:    start.Add(time.Hour),
				Scope:         MaintenanceScopeResources,
			},
			Resources: resources,
		}
	}

	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	t.Run("conflict present in the snapshot is known", func(t *testing.T) {
		live := []*ConflictWithResources{newConflict(maintA, base)}
		snapshot := []*ConflictWithResources{newConflict(maintA, base)}

		MarkKnownAtApproval(live, snapshot)

		require.True(t, live[0].KnownAtApproval)
	})

	t.Run("conflict absent from the snapshot is new", func(t *testing.T) {
		live := []*ConflictWithResources{newConflict(maintB, base)}
		snapshot := []*ConflictWithResources{newConflict(maintA, base)}

		MarkKnownAtApproval(live, snapshot)

		require.False(t, live[0].KnownAtApproval)
	})

	t.Run("shifted overlap window stays known", func(t *testing.T) {
		// The approver reviewed this neighbor; moving it in time does not make
		// it a conflict nobody has seen. Guards against putting the period back
		// into the matching key.
		live := []*ConflictWithResources{newConflict(maintA, base.Add(4*time.Hour))}
		snapshot := []*ConflictWithResources{newConflict(maintA, base)}

		MarkKnownAtApproval(live, snapshot)

		require.True(t, live[0].KnownAtApproval)
	})

	t.Run("snapshot without resources still matches", func(t *testing.T) {
		// Snapshots written by the real UI carry no resources at all; keying on
		// them would flag every resource-scoped conflict as new.
		live := []*ConflictWithResources{newConflict(maintA, base, resID1, resID2)}
		snapshot := []*ConflictWithResources{newConflict(maintA, base)}

		MarkKnownAtApproval(live, snapshot)

		require.True(t, live[0].KnownAtApproval)
	})

	t.Run("empty snapshot leaves everything new", func(t *testing.T) {
		live := []*ConflictWithResources{newConflict(maintA, base), newConflict(maintB, base)}

		MarkKnownAtApproval(live, nil)

		require.False(t, live[0].KnownAtApproval)
		require.False(t, live[1].KnownAtApproval)
	})

	t.Run("empty live set is not a panic", func(t *testing.T) {
		require.NotPanics(t, func() {
			MarkKnownAtApproval(nil, []*ConflictWithResources{newConflict(maintA, base)})
		})
	})
}

func TestSortConflicts(t *testing.T) {
	early := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	late := time.Date(2026, 3, 1, 18, 0, 0, 0, time.UTC)

	id1 := uuid.MustParse("11111111-0000-0000-0000-000000000001")
	id2 := uuid.MustParse("22222222-0000-0000-0000-000000000002")

	newConflict := func(id uuid.UUID, start time.Time, known bool) *ConflictWithResources {
		return &ConflictWithResources{
			Conflict: &Conflict{
				MaintenanceID: id,
				OverlapStart:  start,
				OverlapEnd:    start.Add(time.Hour),
				Scope:         MaintenanceScopeGlobal,
			},
			KnownAtApproval: known,
		}
	}

	t.Run("unknown conflicts come first", func(t *testing.T) {
		conflicts := []*ConflictWithResources{
			newConflict(id1, early, true),
			newConflict(id2, late, false),
		}

		SortConflicts(conflicts)

		require.False(t, conflicts[0].KnownAtApproval, "the conflict nobody reviewed must lead")
		require.True(t, conflicts[1].KnownAtApproval)
	})

	t.Run("within a group the earlier overlap wins", func(t *testing.T) {
		conflicts := []*ConflictWithResources{
			newConflict(id1, late, false),
			newConflict(id2, early, false),
		}

		SortConflicts(conflicts)

		require.Equal(t, early, conflicts[0].OverlapStart)
	})

	t.Run("maintenance id is the final tie-breaker", func(t *testing.T) {
		conflicts := []*ConflictWithResources{
			newConflict(id2, early, false),
			newConflict(id1, early, false),
		}

		SortConflicts(conflicts)

		require.Equal(t, id1, conflicts[0].MaintenanceID)
	})

	t.Run("every permutation lands on the one expected order", func(t *testing.T) {
		// id1 and id2 share `early` within the unreviewed group, so the id
		// tie-breaker — the third axis — actually decides a position here. A
		// comparator reversed on any single axis fails this, which comparing two
		// runs against each other would not catch.
		type want struct {
			id    uuid.UUID
			start time.Time
			known bool
		}
		expected := []want{
			{id1, early, false},
			{id2, early, false},
			{id1, late, false},
			{id2, early, true},
			{id1, late, true},
		}

		build := func() []*ConflictWithResources {
			return []*ConflictWithResources{
				newConflict(id1, late, true),
				newConflict(id2, early, false),
				newConflict(id1, late, false),
				newConflict(id1, early, false),
				newConflict(id2, early, true),
			}
		}

		permutations := [][]int{
			{0, 1, 2, 3, 4},
			{4, 3, 2, 1, 0},
			{2, 0, 4, 1, 3},
		}

		for _, order := range permutations {
			source := build()
			shuffled := make([]*ConflictWithResources, 0, len(source))
			for _, idx := range order {
				shuffled = append(shuffled, source[idx])
			}

			SortConflicts(shuffled)

			require.Len(t, shuffled, len(expected))
			for i, w := range expected {
				require.Equal(t, w.known, shuffled[i].KnownAtApproval, "position %d, input order %v", i, order)
				require.Equal(t, w.start, shuffled[i].OverlapStart, "position %d, input order %v", i, order)
				require.Equal(t, w.id, shuffled[i].MaintenanceID, "position %d, input order %v", i, order)
			}
		}
	})
}

func TestConflictFingerprint_IgnoresKnownAtApproval(t *testing.T) {
	// The fingerprint gates approve (checkConflicts). If the read-side flag ever
	// leaked into it, approving would start failing with a spurious
	// "conflicts changed since preview".
	build := func(known bool) []*ConflictWithResources {
		return []*ConflictWithResources{
			{
				Conflict: &Conflict{
					MaintenanceID: uuid.MustParse("cccccccc-0000-0000-0000-000000000003"),
					Title:         "conflict",
					OverlapStart:  time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
					OverlapEnd:    time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC),
					Scope:         MaintenanceScopeGlobal,
				},
				Resources:       []uuid.UUID{resID1},
				KnownAtApproval: known,
			},
		}
	}

	require.Equal(t, ConflictFingerprint(build(false)), ConflictFingerprint(build(true)))
}

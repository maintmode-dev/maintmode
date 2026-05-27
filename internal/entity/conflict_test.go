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

func TestConflictFingerprint(t *testing.T) {
	tests := []struct {
		name      string
		conflicts []*ConflictWithResources
	}{
		{
			name:      "empty conflicts list",
			conflicts: nil,
		},
		{
			name:      "empty conflicts slice",
			conflicts: []*ConflictWithResources{},
		},
		{
			name: "single conflict with no resources",
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
			name: "single conflict with one resource",
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
		{
			name: "single conflict with multiple resources",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: []uuid.UUID{resID1, resID2, resID3},
				},
			},
		},
		{
			name: "conflicts with resources in different order (should produce same fingerprint)",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: []uuid.UUID{resID3, resID1, resID2},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConflictFingerprint(tt.conflicts)
			require.NotEmpty(t, got)
			require.Len(t, got, 64)
		})
	}
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

func TestConflictResourcesFingerprint(t *testing.T) {
	tests := []struct {
		name      string
		resources []uuid.UUID
	}{
		{name: "empty slice", resources: []uuid.UUID{}},
		{name: "nil slice", resources: nil},
		{name: "single resource", resources: []uuid.UUID{resID1}},
		{name: "multiple resources", resources: []uuid.UUID{resID1, resID2, resID3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := conflictResourcesFingerprint(tt.resources)
			if len(tt.resources) == 0 {
				require.Empty(t, got)
			} else {
				require.NotEmpty(t, got)
			}
		})
	}
}

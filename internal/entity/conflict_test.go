package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestConflictFingerprint(t *testing.T) {
	// Test cases for ConflictFingerprint function
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
					Resources: []*Resource{
						{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
					},
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
					Resources: []*Resource{
						{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
						{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
						{ID: uuid.MustParse("30000000-0000-0000-0000-000000000003"), Type: ResourceTypeCluster},
					},
				},
			},
		},
		{
			name: "multiple conflicts with different maintenance IDs",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeGlobal,
					},
					Resources: []*Resource{
						{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
					},
				},
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
						Title:         "Test Conflict 2",
						OverlapStart:  time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: []*Resource{
						{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
					},
				},
			},
		},
		{
			name: "conflicts with same maintenance ID but different scopes",
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
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 2",
						OverlapStart:  time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: []*Resource{
						{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
					},
				},
			},
		},
		{
			name: "conflicts with same maintenance ID and scope but different overlap times",
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
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 2",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeGlobal,
					},
					Resources: nil,
				},
			},
		},
		{
			name: "conflicts with different time zones (should be normalized to UTC)",
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
			name: "conflicts with milliseconds (should be truncated to seconds)",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 500000000, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 750000000, time.UTC),
						Scope:         MaintenanceScopeGlobal,
					},
					Resources: nil,
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
					Resources: []*Resource{
						{ID: uuid.MustParse("30000000-0000-0000-0000-000000000003"), Type: ResourceTypeCluster},
						{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
						{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
					},
				},
			},
		},
		{
			name: "conflicts in different order (should produce same fingerprint)",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
						Title:         "Test Conflict 2",
						OverlapStart:  time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: []*Resource{
						{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
					},
				},
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeGlobal,
					},
					Resources: []*Resource{
						{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
					},
				},
			},
		},
		{
			name: "conflict with empty resources slice",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: []*Resource{},
				},
			},
		},
		{
			name: "conflicts with different resource types",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: []*Resource{
						{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
						{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
						{ID: uuid.MustParse("30000000-0000-0000-0000-000000000003"), Type: ResourceTypeCluster},
					},
				},
			},
		},
		{
			name: "conflicts with same resources but different order",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeResources,
					},
					Resources: []*Resource{
						{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
						{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConflictFingerprint(tt.conflicts)
			require.NotEmpty(t, got, "ConflictFingerprint() should return a non-empty hash")
			require.Len(t, got, 64, "ConflictFingerprint() should return a 64-character SHA256 hash")
		})
	}
}

func TestConflictFingerprint_Deterministic(t *testing.T) {
	// Test that the same input produces the same fingerprint multiple times
	conflicts := []*ConflictWithResources{
		{
			Conflict: &Conflict{
				MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				Title:         "Test Conflict 1",
				OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Scope:         MaintenanceScopeResources,
			},
			Resources: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
				{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
			},
		},
	}

	// Call the function multiple times and verify it returns the same result
	fingerprints := make([]string, 10)
	for i := 0; i < 10; i++ {
		fingerprints[i] = ConflictFingerprint(conflicts)
	}

	// All fingerprints should be identical
	for i := 1; i < len(fingerprints); i++ {
		require.Equal(t, fingerprints[0], fingerprints[i], "ConflictFingerprint() should be deterministic")
	}
}

func TestConflictFingerprint_DifferentInputs(t *testing.T) {
	// Test that different inputs produce different fingerprints
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
			name: "different overlap start",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeGlobal,
					},
					Resources: nil,
				},
			},
		},
		{
			name: "different overlap end",
			conflicts: []*ConflictWithResources{
				{
					Conflict: &Conflict{
						MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
						Title:         "Test Conflict 1",
						OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
						OverlapEnd:    time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
						Scope:         MaintenanceScopeGlobal,
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
					Resources: []*Resource{
						{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
					},
				},
			},
		},
	}

	fingerprints := make(map[string]bool)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fp := ConflictFingerprint(tc.conflicts)
			require.NotContains(t, fingerprints, fp, "fingerprint should be unique for different inputs")
			fingerprints[fp] = true
		})
	}
}

func TestConflictFingerprint_OrderIndependence(t *testing.T) {
	// Test that the order of conflicts and resources doesn't affect the fingerprint
	baseConflicts := []*ConflictWithResources{
		{
			Conflict: &Conflict{
				MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				Title:         "Test Conflict 1",
				OverlapStart:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				OverlapEnd:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Scope:         MaintenanceScopeResources,
			},
			Resources: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
				{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
			},
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

	// Create a copy with reversed order
	reversedConflicts := make([]*ConflictWithResources, len(baseConflicts))
	copy(reversedConflicts, baseConflicts)
	for i, j := 0, len(reversedConflicts)-1; i < j; i, j = i+1, j-1 {
		reversedConflicts[i], reversedConflicts[j] = reversedConflicts[j], reversedConflicts[i]
	}

	// Also reverse the resources within each conflict
	for _, c := range reversedConflicts {
		if c != nil && len(c.Resources) > 0 {
			for i, j := 0, len(c.Resources)-1; i < j; i, j = i+1, j-1 {
				c.Resources[i], c.Resources[j] = c.Resources[j], c.Resources[i]
			}
		}
	}

	fp1 := ConflictFingerprint(baseConflicts)
	fp2 := ConflictFingerprint(reversedConflicts)

	require.Equal(t, fp1, fp2, "fingerprint should be independent of order")
}

func TestConflictFingerprint_TimeTruncation(t *testing.T) {
	// Test that milliseconds are truncated to seconds
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		time time.Time
	}{
		{"no milliseconds", baseTime},
		{"500 milliseconds", baseTime.Add(500 * time.Millisecond)},
		{"999 milliseconds", baseTime.Add(999 * time.Millisecond)},
		{"with nanoseconds", baseTime.Add(123456789)},
	}

	conflicts := make([]*ConflictWithResources, len(tests))
	for i, tt := range tests {
		conflicts[i] = &ConflictWithResources{
			Conflict: &Conflict{
				MaintenanceID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				Title:         "Test Conflict 1",
				OverlapStart:  tt.time,
				OverlapEnd:    tt.time.Add(2 * time.Hour),
				Scope:         MaintenanceScopeGlobal,
			},
			Resources: nil,
		}
	}

	// All fingerprints should be the same since times are truncated to seconds
	fingerprints := make([]string, len(conflicts))
	for i, c := range conflicts {
		fingerprints[i] = ConflictFingerprint([]*ConflictWithResources{c})
	}

	// All fingerprints should be identical
	for i := 1; i < len(fingerprints); i++ {
		require.Equal(t, fingerprints[0], fingerprints[i], "ConflictFingerprint() should truncate milliseconds")
	}
}

func TestSortResources(t *testing.T) {
	tests := []struct {
		name      string
		resources []*Resource
		want      []*Resource
	}{
		{
			name:      "empty slice",
			resources: []*Resource{},
			want:      []*Resource{},
		},
		{
			name:      "nil slice",
			resources: nil,
			want:      nil,
		},
		{
			name: "already sorted",
			resources: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
				{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
			},
			want: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
				{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
			},
		},
		{
			name: "reverse order",
			resources: []*Resource{
				{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
			},
			want: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
				{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
			},
		},
		{
			name: "same ID, different types",
			resources: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeDatabase},
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeCluster},
			},
			want: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeCluster},
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeDatabase},
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
			},
		},
		{
			name: "multiple resources with same ID",
			resources: []*Resource{
				{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeDatabase},
				{ID: uuid.MustParse("30000000-0000-0000-0000-000000000003"), Type: ResourceTypeCluster},
			},
			want: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeDatabase},
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
				{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
				{ID: uuid.MustParse("30000000-0000-0000-0000-000000000003"), Type: ResourceTypeCluster},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortResources(tt.resources)
			require.Equal(t, tt.want, tt.resources, "SortResources() should sort resources correctly")
		})
	}
}

func TestConflictResourcesFingerprint(t *testing.T) {
	tests := []struct {
		name      string
		resources []*Resource
	}{
		{
			name:      "empty slice",
			resources: []*Resource{},
		},
		{
			name:      "nil slice",
			resources: nil,
		},
		{
			name: "single resource",
			resources: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
			},
		},
		{
			name: "multiple resources",
			resources: []*Resource{
				{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Type: ResourceTypeService},
				{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), Type: ResourceTypeDatabase},
				{ID: uuid.MustParse("30000000-0000-0000-0000-000000000003"), Type: ResourceTypeCluster},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := conflictResourcesFingerprint(tt.resources)
			if len(tt.resources) == 0 {
				require.Empty(t, got, "conflictResourcesFingerprint() should return empty for empty resources")
			} else {
				require.NotEmpty(t, got, "conflictResourcesFingerprint() should return non-empty for non-empty resources")
			}
		})
	}
}

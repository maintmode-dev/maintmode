package auditor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// facetsFromActionCounts routes per-action counts into the FE category chips.
// A maintenance action and an auth action must land in their own buckets, and
// All must total every count regardless of category.
func TestFacetsFromActionCounts_RoutesByCategory(t *testing.T) {
	counts := map[entity.AuditAction]int64{
		entity.AuditActionMaintStarted: 3,
		entity.AuditActionLoginSuccess: 2,
	}

	facets := facetsFromActionCounts(context.Background(), counts)

	require.Equal(t, int64(3), facets.Maintenance)
	require.Equal(t, int64(2), facets.Auth)
	require.Equal(t, int64(5), facets.All)
	require.Zero(t, facets.Roles)
	require.Zero(t, facets.Block)
}

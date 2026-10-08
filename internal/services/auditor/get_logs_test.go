package auditor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

// facetsFromActionCounts routes per-action counts into the FE category chips.
// Each category gets actions from more than one former chip, so a count that
// still followed the old grouping (password/provider events with sign-ins,
// auth_method.toggled away from integrations) lands in the wrong bucket here.
// All must total every count regardless of category, and with every action
// categorized the four buckets sum to All.
func TestFacetsFromActionCounts_RoutesByCategory(t *testing.T) {
	t.Parallel()

	counts := map[entity.AuditAction]int64{
		entity.AuditActionLoginSuccess:       2,
		entity.AuditActionLoginFailed:        1,
		entity.AuditActionPasswordReset:      4,
		entity.AuditActionProviderLinked:     8,
		entity.AuditActionUserBlocked:        16,
		entity.AuditActionAuthMethodToggled:  32,
		entity.AuditActionIntegrationCreated: 64,
		entity.AuditActionMaintStarted:       128,
		entity.AuditActionMaintStepCanceled:  256,
	}

	facets := facetsFromActionCounts(context.Background(), counts)

	require.Equal(t, entity.AuditFacets{
		All:         511,
		SignIn:      3,
		Users:       28,
		Settings:    96,
		Maintenance: 384,
	}, facets)
	require.Equal(t, facets.All, facets.SignIn+facets.Users+facets.Settings+facets.Maintenance)
}

// An action outside every category still counts toward All -- the UI renders
// rows it does not know under the All chip -- and toward no category.
func TestFacetsFromActionCounts_UnknownActionCountsOnlyTowardAll(t *testing.T) {
	t.Parallel()

	facets := facetsFromActionCounts(context.Background(), map[entity.AuditAction]int64{
		entity.AuditActionLogoutSuccess: 1,
		"future.action":                 5,
	})

	require.Equal(t, entity.AuditFacets{All: 6, SignIn: 1}, facets)
}

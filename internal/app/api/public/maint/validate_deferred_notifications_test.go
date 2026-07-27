package apimaint

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
)

// validUpdateRequest builds an otherwise-valid update payload so the assertions
// below isolate deferred_notifications.
func validUpdateRequest(deferred *[]*apimodels.DeferredNotification) *apimodels.UpdateDraftMaintRequest {
	return &apimodels.UpdateDraftMaintRequest{
		Title:        "Updated title",
		Description:  "Updated description",
		PlannedStart: time.Now().Add(48 * time.Hour),
		Scope:        apimodels.MaintenanceScopeGlobal,
		Impact:       apimodels.MaintenanceImpactFull,
		Steps: []*apimodels.MaintenanceStepInput{{
			Order:               1,
			Description:         "Step",
			RollbackDescription: "Rollback",
			Duration:            "45m",
		}},
		NotifyTargets: &apimodels.NotifyTargets{
			ChannelIDs: []string{"0197a3c1-7a2b-7c3d-9e4f-5a6b7c8d9e0f"},
		},
		DeferredNotifications: deferred,
	}
}

// TestValidateUpdateMaintRequestDeferredNotifications covers the tri-state
// contract. The pointer states must all validate, and a malformed element must
// still be rejected: validation.Each does not dereference a pointer, so an
// oversight there would both break every edit and silently stop checking
// fire_at.
func TestValidateUpdateMaintRequestDeferredNotifications(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("accepts all three states", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name     string
			deferred *[]*apimodels.DeferredNotification
		}{
			{
				name:     "absent means unchanged",
				deferred: nil,
			}, {
				name:     "empty means clear",
				deferred: lo.ToPtr([]*apimodels.DeferredNotification{}),
			}, {
				name: "non-empty means replace",
				deferred: lo.ToPtr([]*apimodels.DeferredNotification{
					{FireAt: time.Now().Add(time.Hour)},
				}),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				err := validateUpdateMaintRequest(ctx, validUpdateRequest(tc.deferred))
				require.NoError(t, err)
			})
		}
	})

	t.Run("rejects an element without fire_at", func(t *testing.T) {
		t.Parallel()

		req := validUpdateRequest(lo.ToPtr([]*apimodels.DeferredNotification{
			{FireAt: time.Time{}},
		}))

		err := validateUpdateMaintRequest(ctx, req)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "must be an iterable")
	})
}

// TestValidateCreateMaintDraftRequestDeferredNotifications guards the create
// path against regressions from splitting the bind helper: create stays on a
// flat slice and a non-empty set must remain valid.
func TestValidateCreateMaintDraftRequestDeferredNotifications(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	req := &apimodels.CreateDraftMaintRequest{
		Title:        "Title",
		Description:  "Description",
		PlannedStart: time.Now().Add(48 * time.Hour),
		Scope:        apimodels.MaintenanceScopeGlobal,
		Impact:       apimodels.MaintenanceImpactFull,
		Steps: []*apimodels.MaintenanceStepInput{{
			Order:               1,
			Description:         "Step",
			RollbackDescription: "Rollback",
			Duration:            "45m",
		}},
		NotifyTargets: &apimodels.NotifyTargets{
			ChannelIDs: []string{"0197a3c1-7a2b-7c3d-9e4f-5a6b7c8d9e0f"},
		},
		DeferredNotifications: []*apimodels.DeferredNotification{
			{FireAt: time.Now().Add(time.Hour)},
		},
		ApproverUserID: uuid.New(),
	}

	require.NoError(t, validateCreateMaintDraftRequest(ctx, req))
}

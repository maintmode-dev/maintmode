package apimaint

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// TestValidateApproveDraftMaintRequest_ResourceScopedConflictRequiresResources
// pins the rule that a resource-scoped conflict must carry resources.
//
// This rule used to be unsatisfiable from the client's side. The conflict list a
// client sends back in conflicts_snapshot is not composed by the client: it
// echoes what GET /ui/v1/maintenances/{id} returned, and the read path used to
// fill `resources` with the INTERSECTION between the conflicting maintenance's
// resources and the viewed maintenance's own. A global-scope draft holds no
// resources, so that intersection was empty against every neighbor — the server
// emitted `scope: "resource"` with `resources: []` and then rejected the echo of
// its own response.
//
// The read path now reports the neighbor's own resource set, so a resource-scoped
// conflict always carries at least one resource: a resource-scoped maintenance
// cannot be written without them (create and update apply the same rule). The
// validator rule therefore stopped being a contradiction and became a real
// invariant guard — and this test now pins the guard rather than the
// contradiction.
//
// Deliberately free of any database. The end-to-end path that produces a valid
// payload is covered by TestApproveMaint_GlobalDraftWithResourceScopedNeighbor.
func TestValidateApproveDraftMaintRequest_ResourceScopedConflictRequiresResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	now := xtime.UTCNow()

	conflict := func(scope apimodels.MaintenanceScope, resources []*apimodels.ResourceRef) *apimodels.Conflict {
		return &apimodels.Conflict{
			MaintenanceID: uuid.New(),
			OverlapStart:  now,
			OverlapEnd:    now.Add(3 * time.Hour),
			Scope:         scope,
			Resources:     resources,
		}
	}

	t.Run("resource-scoped conflict with resources is accepted", func(t *testing.T) {
		t.Parallel()

		err := validateApproveDraftMaintRequest(ctx, &apimodels.ApproveDraftMaintRequest{
			ObservedMaintRevision: now.UnixMicro(),
			ConflictsSnapshot: []*apimodels.Conflict{
				conflict(apimodels.MaintenanceScopeResources, []*apimodels.ResourceRef{{ID: uuid.New()}}),
			},
		})
		require.NoError(t, err,
			"this is the shape the read path now emits for a resource-scoped neighbor")
	})

	t.Run("resource-scoped conflict without resources is rejected", func(t *testing.T) {
		t.Parallel()

		err := validateApproveDraftMaintRequest(ctx, &apimodels.ApproveDraftMaintRequest{
			ObservedMaintRevision: now.UnixMicro(),
			ConflictsSnapshot: []*apimodels.Conflict{
				conflict(apimodels.MaintenanceScopeResources, []*apimodels.ResourceRef{}),
			},
		})
		require.Error(t, err,
			"a resource-scoped maintenance cannot exist without resources, so this "+
				"payload is malformed rather than merely unusual")
	})

	// The one case where an empty list stays legal: a global-scope neighbor owns
	// no resources at all, so the rule must not fire for it. Without this, the
	// original bug would simply move rather than be fixed.
	t.Run("global-scoped conflict without resources is accepted", func(t *testing.T) {
		t.Parallel()

		err := validateApproveDraftMaintRequest(ctx, &apimodels.ApproveDraftMaintRequest{
			ObservedMaintRevision: now.UnixMicro(),
			ConflictsSnapshot: []*apimodels.Conflict{
				conflict(apimodels.MaintenanceScopeGlobal, []*apimodels.ResourceRef{}),
			},
		})
		require.NoError(t, err,
			"a global-scope maintenance holds no resources; requiring them here would "+
				"recreate the unapprovable-draft bug on a different path")
	})
}

// TestValidateApproveDraftMaintRequest_ConflictInvariants guards the rules that
// must keep holding while the resources rule above is relaxed, so a fix for the
// empty-intersection case cannot quietly turn the whole conflict element into an
// unvalidated blob.
func TestValidateApproveDraftMaintRequest_ConflictInvariants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	now := xtime.UTCNow()

	validConflict := func() *apimodels.Conflict {
		return &apimodels.Conflict{
			MaintenanceID: uuid.New(),
			OverlapStart:  now,
			OverlapEnd:    now.Add(time.Hour),
			Scope:         apimodels.MaintenanceScopeGlobal,
			Resources:     []*apimodels.ResourceRef{},
		}
	}

	for _, tc := range []struct {
		name    string
		mutate  func(c *apimodels.Conflict)
		wantErr bool
	}{
		{
			name:    "valid global conflict",
			mutate:  func(_ *apimodels.Conflict) {},
			wantErr: false,
		},
		{
			name:    "missing maintenance id",
			mutate:  func(c *apimodels.Conflict) { c.MaintenanceID = uuid.Nil },
			wantErr: true,
		},
		{
			name:    "missing overlap start",
			mutate:  func(c *apimodels.Conflict) { c.OverlapStart = time.Time{} },
			wantErr: true,
		},
		{
			name:    "missing overlap end",
			mutate:  func(c *apimodels.Conflict) { c.OverlapEnd = time.Time{} },
			wantErr: true,
		},
		{
			name:    "missing scope",
			mutate:  func(c *apimodels.Conflict) { c.Scope = "" },
			wantErr: true,
		},
		{
			// A resource entry that IS present must still carry a usable id:
			// relaxing "the list may be empty" must not become "the list may
			// contain junk".
			name: "resource entry without id",
			mutate: func(c *apimodels.Conflict) {
				c.Scope = apimodels.MaintenanceScopeResources
				c.Resources = []*apimodels.ResourceRef{{ID: uuid.Nil}}
			},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			conflict := validConflict()
			tc.mutate(conflict)

			err := validateApproveDraftMaintRequest(ctx, &apimodels.ApproveDraftMaintRequest{
				ObservedMaintRevision: now.UnixMicro(),
				ConflictsSnapshot:     []*apimodels.Conflict{conflict},
			})

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

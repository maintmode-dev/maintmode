package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
)

func TestConflictedResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := xtime.UTCNow()
	start, end := now.Add(time.Hour), now.Add(3*time.Hour)
	store := conflicts.NewStore(db)

	t.Run("has overlap", func(t *testing.T) {
		t.Parallel()
		sharedResource := testdbutils.MakeResource(ctx, t, resourcesStore)

		for _, tc := range []struct {
			name                        string
			maint                       *entity.Maintenance
			conflictedMaint             *entity.Maintenance
			expectedConflictedResources func(conflictedMaint *entity.Maintenance) map[uuid.UUID][]uuid.UUID
		}{
			{
				name: "conflicted maint has global scope",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
				),
				conflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
					testdbutils.WithScope(entity.MaintenanceScopeGlobal),
				),
				expectedConflictedResources: func(_ *entity.Maintenance) map[uuid.UUID][]uuid.UUID {
					return map[uuid.UUID][]uuid.UUID{}
				},
			}, {
				name: "maint has global scope",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeGlobal),
				),
				conflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
				),
				expectedConflictedResources: func(_ *entity.Maintenance) map[uuid.UUID][]uuid.UUID {
					return map[uuid.UUID][]uuid.UUID{}
				},
			}, {
				name: "has conflicted resources",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
					testdbutils.WithResources(
						sharedResource.ID,
						testdbutils.MakeResource(ctx, t, resourcesStore).ID,
					),
				),
				conflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
					testdbutils.WithResources(
						sharedResource.ID,
						testdbutils.MakeResource(ctx, t, resourcesStore).ID,
					),
				),
				expectedConflictedResources: func(conflictedMaint *entity.Maintenance) map[uuid.UUID][]uuid.UUID {
					return map[uuid.UUID][]uuid.UUID{
						conflictedMaint.ID: {sharedResource.ID},
					}
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				actualConflictedResources, err := store.ConflictedResources(ctx, &entity.ConflictResourcesQueryCmd{
					MaintResourceIDs:   tc.maint.Resources,
					ConflictedMaintIDs: []uuid.UUID{tc.conflictedMaint.ID},
				})
				require.NoError(t, err)

				expectedConflictedResources := tc.expectedConflictedResources(tc.conflictedMaint)
				require.Equal(t, len(expectedConflictedResources), len(actualConflictedResources))

				for conflictedMaintID, expectedResources := range expectedConflictedResources {
					actualResources, ok := actualConflictedResources[conflictedMaintID]
					require.Truef(t, ok, "not found conflict with id %s", conflictedMaintID)
					require.Equal(t, expectedResources, actualResources)
				}
			})
		}
	})

	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()

		maint := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(testdbutils.MakeResource(ctx, t, resourcesStore).ID),
		)
		notConflictedMaint := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(testdbutils.MakeResource(ctx, t, resourcesStore).ID),
		)

		actualConflictedResources, err := store.ConflictedResources(ctx, &entity.ConflictResourcesQueryCmd{
			MaintResourceIDs:   maint.Resources,
			ConflictedMaintIDs: []uuid.UUID{notConflictedMaint.ID},
		})
		require.NoError(t, err)
		require.Equal(t, 0, len(actualConflictedResources))
	})
}

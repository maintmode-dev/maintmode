package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
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
		sharedResource := &entity.Resource{
			ID:   xuuid.New(),
			Type: entity.ResourceTypeDatabase,
		}

		for _, tc := range []struct {
			name                        string
			maint                       *entity.Maintenance
			conflictedMaint             *entity.Maintenance
			expectedConflictedResources func(conflictedMaint *entity.Maintenance) map[uuid.UUID][]*entity.Resource
		}{
			{
				name: "conflicted maint has global scope",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
				),
				conflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore,
					entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
					testdbutils.WithScope(entity.MaintenanceScopeGlobal),
				),
				expectedConflictedResources: func(_ *entity.Maintenance) map[uuid.UUID][]*entity.Resource {
					return map[uuid.UUID][]*entity.Resource{}
				},
			}, {
				name: "maint has global scope",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeGlobal),
				),
				conflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore,
					entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
				),
				expectedConflictedResources: func(_ *entity.Maintenance) map[uuid.UUID][]*entity.Resource {
					return map[uuid.UUID][]*entity.Resource{}
				},
			}, {
				name: "has conflicted resources",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
					testdbutils.WithResources(sharedResource, &entity.Resource{
						ID:   xuuid.New(),
						Type: entity.ResourceTypeService,
					}),
				),
				conflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore,
					entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
					testdbutils.WithResources(sharedResource, &entity.Resource{
						ID:   xuuid.New(),
						Type: entity.ResourceTypeService,
					}),
				),
				expectedConflictedResources: func(conflictedMaint *entity.Maintenance) map[uuid.UUID][]*entity.Resource {
					return map[uuid.UUID][]*entity.Resource{
						conflictedMaint.ID: {
							sharedResource,
						},
					}
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				actualConflictedResources, err := store.ConflictedResources(ctx, &entity.ConflictResourcesQueryCmd{
					MaintResourceIDs: lo.Map(tc.maint.Resources, func(item *entity.Resource, _ int) uuid.UUID {
						return item.ID
					}),
					ConflictedMaintIDs: []uuid.UUID{tc.conflictedMaint.ID},
				})
				require.NoError(t, err)

				expectedConflictedResources := tc.expectedConflictedResources(tc.conflictedMaint)
				require.Equal(t, len(expectedConflictedResources), len(actualConflictedResources))

				for conflictedMaindID, expectedResources := range expectedConflictedResources {
					actualResources, ok := actualConflictedResources[conflictedMaindID]
					require.Truef(t, ok, "not found conflict with id %s", conflictedMaindID)
					require.Equal(t, expectedResources, actualResources)
				}
			})
		}
	})

	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()

		maint := testdbutils.MakeMaint(ctx, t, maintsStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(&entity.Resource{ID: xuuid.New(), Type: entity.ResourceTypeService}),
		)
		notConflictedMaint := testdbutils.MakeMaint(ctx, t, maintsStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(&entity.Resource{ID: xuuid.New(), Type: entity.ResourceTypeService}),
		)

		actualConflictedResources, err := store.ConflictedResources(ctx, &entity.ConflictResourcesQueryCmd{
			MaintResourceIDs: lo.Map(maint.Resources, func(item *entity.Resource, _ int) uuid.UUID {
				return item.ID
			}),
			ConflictedMaintIDs: []uuid.UUID{notConflictedMaint.ID},
		})
		require.NoError(t, err)
		require.Equal(t, 0, len(actualConflictedResources))
	})
}

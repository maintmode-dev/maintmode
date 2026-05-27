package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testtimeutils "github.com/ruko1202/maintmode/test/utils/time"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/conflicts"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

func TestListConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := xtime.UTCNow()
	start, end := now.Add(time.Hour), now.Add(3*time.Hour)
	store := conflicts.NewStore(db)

	t.Run("has overlap", func(t *testing.T) {
		t.Parallel()
		sharedResource := testdbutils.MakeResource(ctx, t, resourcesStore)

		for _, tc := range []struct {
			name            string
			maint           *entity.Maintenance
			conflictedMaint *entity.Maintenance
		}{
			{
				name: "conflicted maint has global scope",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
				),
				conflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
					testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
					testdbutils.WithScope(entity.MaintenanceScopeGlobal),
				),
			}, {
				name: "maint has global scope",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeGlobal),
				),
				conflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
					testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
				),
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
					testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
					testdbutils.WithResources(
						sharedResource.ID,
						testdbutils.MakeResource(ctx, t, resourcesStore).ID,
					),
				),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				actualConflictedMaints, err := store.ConflictedMaints(ctx, &entity.ConflictQueryCmd{
					MaintID:       tc.maint.ID,
					Scope:         tc.maint.Scope,
					PlannedPeriod: tc.maint.PlannedPeriod,
					ResourceIDs:   tc.maint.Resources,
				})
				require.NoError(t, err)
				require.NotEmpty(t, actualConflictedMaints)
				actualConflictedMaintsM := lo.SliceToMap(actualConflictedMaints, func(item *entity.Conflict) (uuid.UUID, *entity.Conflict) {
					return item.MaintenanceID, item
				})
				for _, expectedConflict := range []*entity.Conflict{{
					MaintenanceID: tc.conflictedMaint.ID,
					Title:         tc.conflictedMaint.Title,
					Scope:         tc.conflictedMaint.Scope,
					OverlapStart:  testtimeutils.OverlapStart(tc.maint.PlannedPeriod, tc.conflictedMaint.PlannedPeriod),
					OverlapEnd:    testtimeutils.OverlapEnd(tc.maint.PlannedPeriod, tc.conflictedMaint.PlannedPeriod),
				}} {
					actualConflict, ok := actualConflictedMaintsM[expectedConflict.MaintenanceID]
					require.Truef(t, ok, "not found conflict with id %s", expectedConflict.MaintenanceID)
					require.Equal(t, expectedConflict, actualConflict)
				}
			})
		}
	})

	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name               string
			maint              *entity.Maintenance
			notConflictedMaint *entity.Maintenance
		}{
			{
				name: "by period",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
				),
				notConflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(end, end.Add(time.Hour)),
					testdbutils.WithScope(entity.MaintenanceScopeGlobal),
				),
			}, {
				name: "by resources",
				maint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
					testdbutils.WithResources(testdbutils.MakeResource(ctx, t, resourcesStore).ID),
				),
				notConflictedMaint: testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
					entity.NewPeriod(start, end),
					testdbutils.WithScope(entity.MaintenanceScopeResources),
					testdbutils.WithResources(testdbutils.MakeResource(ctx, t, resourcesStore).ID),
				),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				actualConflictedMaints, err := store.ConflictedMaints(ctx, &entity.ConflictQueryCmd{
					MaintID:       tc.maint.ID,
					Scope:         tc.maint.Scope,
					PlannedPeriod: tc.maint.PlannedPeriod,
					ResourceIDs:   tc.maint.Resources,
				})
				require.NoError(t, err)
				require.NotEmpty(t, actualConflictedMaints)
				actualConflictedMaintsM := lo.SliceToMap(actualConflictedMaints, func(item *entity.Conflict) (uuid.UUID, *entity.Conflict) {
					return item.MaintenanceID, item
				})
				for _, expectedNotConflicted := range []*entity.Conflict{{
					MaintenanceID: tc.notConflictedMaint.ID,
				}} {
					actualConflict, ok := actualConflictedMaintsM[expectedNotConflicted.MaintenanceID]
					require.Falsef(t, ok, "found conflict with id %s", expectedNotConflicted.MaintenanceID)
					require.Nil(t, actualConflict)
				}
			})
		}
	})
}

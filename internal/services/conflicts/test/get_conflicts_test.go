package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
	testtimeutils "github.com/ruko1202/maintmode/test/utils/time"
)

func TestGetConflicts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	start, end := now, now.Add(5*time.Hour)

	s := initService(db)

	t.Run("has conflicts", func(t *testing.T) {
		t.Parallel()
		sharedResource := testdbutils.MakeResource(ctx, t, resourcesStore, entity.ResourceTypeService)

		maint := testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(
				sharedResource,
				testdbutils.MakeResource(ctx, t, resourcesStore, entity.ResourceTypeService),
			),
		)

		conflictedMaints := []*entity.Maintenance{
			// using sharedResource
			testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
				entity.NewPeriod(start.Add(time.Hour), end.Add(-time.Hour)),
				testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
				testdbutils.WithScope(entity.MaintenanceScopeResources),
				testdbutils.WithResources(
					sharedResource,
					testdbutils.MakeResource(ctx, t, resourcesStore, entity.ResourceTypeService),
				),
			),
			// resource with the other resource type
			testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
				entity.NewPeriod(start.Add(time.Hour), end.Add(time.Hour)),
				testdbutils.WithStatus(entity.MaintenanceStatusInProgress),
				testdbutils.WithScope(entity.MaintenanceScopeResources),
				testdbutils.WithResources(
					sharedResource,
					&entity.Resource{ID: sharedResource.ID, Type: entity.ResourceTypeDatabase},
				),
			),
			// global scope
			testdbutils.MakeMaint(ctx, t, maintStore, resourcesStore,
				entity.NewPeriod(start, end),
				testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
				testdbutils.WithScope(entity.MaintenanceScopeGlobal),
			),
		}

		actualConflicts, err := s.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs: lo.Map(maint.Resources, func(item *entity.Resource, _ int) uuid.UUID {
				return item.ID
			}),
		})
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(actualConflicts), 3)

		actualConflictsM := lo.SliceToMap(actualConflicts, func(item *entity.ConflictWithResources) (uuid.UUID, *entity.ConflictWithResources) {
			return item.MaintenanceID, item
		})

		expectedConflictsM := lo.SliceToMap(conflictedMaints, func(item *entity.Maintenance) (uuid.UUID, *entity.ConflictWithResources) {
			return item.ID, &entity.ConflictWithResources{
				Conflict: &entity.Conflict{
					MaintenanceID: item.ID,
					Title:         item.Title,
					OverlapStart:  testtimeutils.OverlapStart(maint.PlannedPeriod, item.PlannedPeriod),
					OverlapEnd:    testtimeutils.OverlapEnd(maint.PlannedPeriod, item.PlannedPeriod),
					Scope:         item.Scope,
				},
				Resources: lo.Filter(item.Resources, func(item *entity.Resource, _ int) bool {
					return item.ID == sharedResource.ID
				}),
			}
		})
		for expectedID, expected := range expectedConflictsM {
			actual, ok := actualConflictsM[expectedID]
			require.Truef(t, ok, "not found conflict with id %s", expectedID)
			require.Equal(t, expected.Conflict, actual.Conflict)
			require.Equal(t, len(expected.Resources), len(actual.Resources))

			entity.SortResources(expected.Resources)
			entity.SortResources(actual.Resources)
			require.Equal(t, expected.Resources, actual.Resources)
		}
	})
}

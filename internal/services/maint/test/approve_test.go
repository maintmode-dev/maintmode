package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

func TestApprove(t *testing.T) {
	ctx := context.Background()
	now := xtime.UTCNow()
	start, end := now, now.Add(5*time.Hour)
	s := initService(db)

	t.Run("ok", func(t *testing.T) {
		sharedResource := &entity.Resource{ID: xuuid.New(), Type: entity.ResourceTypeService}

		conflictedMaints := []*entity.Maintenance{
			// using sharedResource
			testdbutils.MakeMaint(ctx, t, maintStore,
				entity.NewPeriod(start.Add(time.Hour), end.Add(-time.Hour)),
				testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
				testdbutils.WithScope(entity.MaintenanceScopeResources),
				testdbutils.WithResources(
					sharedResource,
					&entity.Resource{ID: xuuid.New(), Type: entity.ResourceTypeService},
				),
			),
			// global scope
			testdbutils.MakeMaint(ctx, t, maintStore,
				entity.NewPeriod(start, end),
				testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
				testdbutils.WithScope(entity.MaintenanceScopeGlobal),
			),
		}
		for _, m := range conflictedMaints {
			err := s.StartMaint(ctx, &entity.StartMaintenanceCmd{MaintID: m.ID})
			require.NoError(t, err)
		}

		maint := testdbutils.MakeMaint(ctx, t, maintStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithResources(
				sharedResource,
				&entity.Resource{ID: xuuid.New(), Type: entity.ResourceTypeService},
			),
		)

		actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs: lo.Map(maint.Resources, func(item *entity.Resource, _ int) uuid.UUID {
				return item.ID
			}),
		})
		require.NoError(t, err)

		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.NoError(t, err)

		actualMaint, err := s.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Equal(t, entity.MaintenanceStatusPlanned, actualMaint.Status)
	})

	t.Run("ErrForbiddenStatusTransition", func(t *testing.T) {
		maint := testdbutils.MakeMaint(ctx, t, maintStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		)

		actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs: lo.Map(maint.Resources, func(item *entity.Resource, _ int) uuid.UUID {
				return item.ID
			}),
		})
		require.NoError(t, err)

		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.ErrorIs(t, err, apperr.ErrForbiddenMaintStatusTransition)
	})

	t.Run("change maint revision", func(t *testing.T) {
		maint := testdbutils.MakeMaint(ctx, t, maintStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
		)

		actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs: lo.Map(maint.Resources, func(item *entity.Resource, _ int) uuid.UUID {
				return item.ID
			}),
		})
		require.NoError(t, err)

		err = s.UpdateMaint(ctx, &entity.UpdateMaintenanceCmd{
			MaintID: maint.ID,
		})
		require.NoError(t, err)

		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.ErrorIs(t, err, apperr.ErrMaintChangedSincePreview)
	})

	t.Run("change conflicts fingerprint", func(t *testing.T) {
		maint := testdbutils.MakeMaint(ctx, t, maintStore,
			entity.NewPeriod(start, end),
			testdbutils.WithScope(entity.MaintenanceScopeResources),
		)

		actualConflicts, err := conflictsSrv.GetConflicts(ctx, &entity.ConflictQueryCmd{
			MaintID:       maint.ID,
			PlannedPeriod: maint.PlannedPeriod,
			Scope:         maint.Scope,
			ResourceIDs: lo.Map(maint.Resources, func(item *entity.Resource, _ int) uuid.UUID {
				return item.ID
			}),
		})
		require.NoError(t, err)

		// conflict maint with global scope
		testdbutils.MakeMaint(ctx, t, maintStore,
			entity.NewPeriod(start, end),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
		)

		err = s.ApproveMaint(ctx, &entity.ApproveMaintenanceCmd{
			MaintID:               maint.ID,
			ObservedMaintRevision: maint.Revision(),
			ConflictSnapshot: entity.ConflictsSnapshot{
				Conflicts: actualConflicts,
			},
		})
		require.ErrorIs(t, err, apperr.ErrConflictsChangedSincePreview)
	})
}

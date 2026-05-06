package maint

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testdbutils "github.com/ruko1202/maintmode/test/utils/db"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow()
	s := initService(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Scope:         entity.MaintenanceScopeResources,
			Resources: []*entity.Resource{
				testdbutils.MakeResource(ctx, t, resourcesStore, entity.ResourceTypeService),
			},
			Steps: []*entity.MaintenanceStepInput{
				{
					Order:               1,
					Description:         "Step1" + t.Name(),
					RollbackDescription: "RollbackStep1" + t.Name(),
					DurationMinutes:     mixStepDurationsMin,
				},
				{
					Order:               2,
					Description:         "Step2" + t.Name(),
					RollbackDescription: "RollbackStep2" + t.Name(),
					DurationMinutes:     mixStepDurationsMin,
				},
			},
		}

		maint, err := s.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint)
		require.NotEmpty(t, maint.ID)
		require.Equal(t, cmd.Title, maint.Title)
		require.Equal(t, cmd.Description, maint.Description)
		require.Equal(t, cmd.PlannedPeriod, maint.PlannedPeriod)
		require.Equal(t, cmd.Impact, maint.Impact)
		require.Nil(t, maint.ActualPeriod)
		require.True(t, maint.CreatedAt.After(now.Add(-time.Minute)), "created at should be after `now`")
	})

	t.Run("maints with overlaps", func(t *testing.T) {
		t.Parallel()

		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(2*time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Scope:         entity.MaintenanceScopeResources,
			Resources: []*entity.Resource{
				testdbutils.MakeResource(ctx, t, resourcesStore, entity.ResourceTypeService),
			},
			Steps: []*entity.MaintenanceStepInput{{
				Order:               1,
				Description:         "Step1" + t.Name(),
				RollbackDescription: "RollbackStep1" + t.Name(),
				DurationMinutes:     mixStepDurationsMin,
			}},
		}
		maint1, err := s.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint1)
		require.NotEmpty(t, maint1.ID)

		cmd.Description = "Description2" + t.Name()
		cmd.PlannedPeriod = entity.NewPeriod(now, now.Add(5*time.Hour))
		cmd.Steps = append(cmd.Steps, &entity.MaintenanceStepInput{
			Order:               2,
			Description:         "Step1" + t.Name(),
			RollbackDescription: "RollbackStep1" + t.Name(),
			DurationMinutes:     mixStepDurationsMin,
		})

		maint2, err := s.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint2)
		require.NotEmpty(t, maint2.ID)

		require.NotEqual(t, maint1.ID, maint2.ID)
		require.Equal(t, maint1.PlannedPeriod.Start, maint2.PlannedPeriod.Start)
		require.True(t, maint1.PlannedPeriod.End.Before(lo.FromPtr(maint2.PlannedPeriod.End)))
	})

	t.Run("duplicate resources", func(t *testing.T) {
		t.Parallel()

		resource := testdbutils.MakeResource(ctx, t, resourcesStore, entity.ResourceTypeService)
		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Scope:         entity.MaintenanceScopeResources,
			Resources:     []*entity.Resource{resource, resource},
			Steps: []*entity.MaintenanceStepInput{{
				Order:               1,
				Description:         "Step1" + t.Name(),
				RollbackDescription: "RollbackStep1" + t.Name(),
				DurationMinutes:     mixStepDurationsMin,
			}},
		}

		maint, err := s.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint)
		require.NotEmpty(t, maint.ID)
	})

	t.Run("global scope does not save resources", func(t *testing.T) {
		t.Parallel()

		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Scope:         entity.MaintenanceScopeGlobal,
			Resources: []*entity.Resource{
				testdbutils.MakeResource(ctx, t, resourcesStore, entity.ResourceTypeService),
			},
			Steps: []*entity.MaintenanceStepInput{{
				Order:               1,
				Description:         "Step1" + t.Name(),
				RollbackDescription: "RollbackStep1" + t.Name(),
				DurationMinutes:     mixStepDurationsMin,
			}},
		}

		maint, err := s.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint)
		require.Empty(t, maint.Resources)

		persistedMaint, err := s.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Empty(t, persistedMaint.Resources)
	})
}

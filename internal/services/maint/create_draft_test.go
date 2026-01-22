package maint

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

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
			Resources: []*entity.Resource{{
				ID:   xuuid.New(),
				Type: entity.ResourceTypeService,
			}},
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
		require.True(t, maint.CreatedAt.After(now), "created at should be after `now`")
	})

	t.Run("maints with overlaps", func(t *testing.T) {
		t.Parallel()

		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(2*time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Resources: []*entity.Resource{{
				ID:   xuuid.New(),
				Type: entity.ResourceTypeService,
			}},
		}
		maint1, err := s.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint1)
		require.NotEmpty(t, maint1.ID)

		cmd.Description = "Description2" + t.Name()
		cmd.PlannedPeriod = entity.NewPeriod(now, now.Add(5*time.Hour))

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

		resource := &entity.Resource{
			ID:   xuuid.New(),
			Type: entity.ResourceTypeService,
		}
		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Resources:     []*entity.Resource{resource, resource},
		}

		maint, err := s.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint)
		require.NotEmpty(t, maint.ID)
	})
}

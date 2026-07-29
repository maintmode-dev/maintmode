package maint

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testdbutils "github.com/ruko1202/maintmode/test/utils/db"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow().Round(time.Microsecond)
	service, mocks := initService(t)
	mocks.expectAnyApproverEligible()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		notifyChannel := makeNotifyChannel(ctx, t, service)

		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Scope:         entity.MaintenanceScopeResources,
			Resources:     []uuid.UUID{testdbutils.MakeResource(ctx, t, service.resourcesStore).ID},
			Steps: []*entity.MaintenanceStepInput{
				{
					Order:               1,
					Description:         "Step1" + t.Name(),
					RollbackDescription: "RollbackStep1" + t.Name(),
					DurationMinutes:     minStepDurationsMinutes,
				},
				{
					Order:               2,
					Description:         "Step2" + t.Name(),
					RollbackDescription: "RollbackStep2" + t.Name(),
					DurationMinutes:     minStepDurationsMinutes,
				},
			},
			NotifyTargets: []*entity.NotifyTargetInput{{
				ChannelID: notifyChannel.ID,
			}},
			ApproverUserID: uuid.New(),
		}

		maint, err := service.CreateDraft(ctx, cmd)
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

		notifyChannel := makeNotifyChannel(ctx, t, service)

		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(2*time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Scope:         entity.MaintenanceScopeResources,
			Resources:     []uuid.UUID{testdbutils.MakeResource(ctx, t, service.resourcesStore).ID},
			Steps: []*entity.MaintenanceStepInput{{
				Order:               1,
				Description:         "Step1" + t.Name(),
				RollbackDescription: "RollbackStep1" + t.Name(),
				DurationMinutes:     minStepDurationsMinutes,
			}},
			NotifyTargets: []*entity.NotifyTargetInput{{
				ChannelID: notifyChannel.ID,
			}},
			CreatedByUserID: uuid.New(),
			ApproverUserID:  uuid.New(),
		}
		maint1, err := service.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint1)
		require.NotEmpty(t, maint1.ID)

		cmd.Description = "Description2" + t.Name()
		cmd.PlannedPeriod = entity.NewPeriod(now, now.Add(5*time.Hour))
		cmd.Steps = append(cmd.Steps, &entity.MaintenanceStepInput{
			Order:               2,
			Description:         "Step1" + t.Name(),
			RollbackDescription: "RollbackStep1" + t.Name(),
			DurationMinutes:     minStepDurationsMinutes,
		})

		maint2, err := service.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint2)
		require.NotEmpty(t, maint2.ID)

		require.NotEqual(t, maint1.ID, maint2.ID)
		require.Equal(t, maint1.PlannedPeriod.Start, maint2.PlannedPeriod.Start)
		require.True(t, maint1.PlannedPeriod.End.Before(lo.FromPtr(maint2.PlannedPeriod.End)))
	})

	t.Run("duplicate resources", func(t *testing.T) {
		t.Parallel()

		notifyChannel := makeNotifyChannel(ctx, t, service)

		resourceID := testdbutils.MakeResource(ctx, t, service.resourcesStore).ID
		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Scope:         entity.MaintenanceScopeResources,
			Resources:     []uuid.UUID{resourceID, resourceID},
			Steps: []*entity.MaintenanceStepInput{{
				Order:               1,
				Description:         "Step1" + t.Name(),
				RollbackDescription: "RollbackStep1" + t.Name(),
				DurationMinutes:     minStepDurationsMinutes,
			}},
			NotifyTargets: []*entity.NotifyTargetInput{{
				ChannelID: notifyChannel.ID,
			}},
			ApproverUserID: uuid.New(),
		}

		maint, err := service.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint)
		require.NotEmpty(t, maint.ID)
	})

	t.Run("global scope does not save resources", func(t *testing.T) {
		t.Parallel()

		notifyChannel := makeNotifyChannel(ctx, t, service)
		cmd := &entity.CreateMaintenanceCmd{
			Title:         "Title" + t.Name(),
			Description:   "Description" + t.Name(),
			PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
			Impact:        entity.MaintenanceImpactFull,
			Scope:         entity.MaintenanceScopeGlobal,
			Resources:     []uuid.UUID{testdbutils.MakeResource(ctx, t, service.resourcesStore).ID},
			Steps: []*entity.MaintenanceStepInput{{
				Order:               1,
				Description:         "Step1" + t.Name(),
				RollbackDescription: "RollbackStep1" + t.Name(),
				DurationMinutes:     minStepDurationsMinutes,
			}},
			NotifyTargets: []*entity.NotifyTargetInput{{
				ChannelID: notifyChannel.ID,
			}},
			ApproverUserID: uuid.New(),
		}

		maint, err := service.CreateDraft(ctx, cmd)
		require.NoError(t, err)
		require.NotNil(t, maint)
		require.Empty(t, maint.Resources)

		persistedMaint, err := service.GetMaint(ctx, maint.ID)
		require.NoError(t, err)
		require.Empty(t, persistedMaint.Resources)
	})
}

// TestCreateDraftMentionsDeduplicates pins that naming the same person twice is
// accepted and collapses to one entry.
//
// The assertion is on the returned maintenance, which CreateDraft fills from
// memory rather than re-reading: the store swallows the second row through
// ON CONFLICT DO NOTHING, so a read-back could not tell whether the dedup
// happened. It matters because this slice is what the create response carries
// and what the renderer turns into the mention line — undeduplicated, the
// person appears twice in one message.
func TestCreateDraftMentionsDeduplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := xtime.UTCNow().Round(time.Microsecond)
	service, mocks := initService(t)
	mocks.expectAnyApproverEligible()

	notifyChannel := makeNotifyChannel(ctx, t, service)
	duplicated := uuid.New()

	maint, err := service.CreateDraft(ctx, &entity.CreateMaintenanceCmd{
		Title:         "Title" + t.Name(),
		Description:   "Description" + t.Name(),
		PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
		Impact:        entity.MaintenanceImpactFull,
		Scope:         entity.MaintenanceScopeGlobal,
		Steps: []*entity.MaintenanceStepInput{{
			Order:               1,
			Description:         "Step1" + t.Name(),
			RollbackDescription: "RollbackStep1" + t.Name(),
			DurationMinutes:     minStepDurationsMinutes,
		}},
		NotifyTargets:  []*entity.NotifyTargetInput{{ChannelID: notifyChannel.ID}},
		Mentions:       []*entity.MentionInput{{UserID: duplicated}, {UserID: duplicated}},
		ApproverUserID: uuid.New(),
	})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{duplicated}, maint.Mentions)
}

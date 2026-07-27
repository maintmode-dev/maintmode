package maint

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testdbutils "github.com/ruko1202/maintmode/test/utils/db"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestUpdateDraft(t *testing.T) {
	t.Parallel()
	now := xtime.UTCNow().Round(time.Microsecond)

	ctx := context.Background()
	defaultPeriod := entity.NewPeriod(now, now.Add(time.Hour))
	service, _ := initService(t)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		notifyChannel := makeNotifyChannel(ctx, t, service)

		for _, tc := range []struct {
			name           string
			cmd            *entity.UpdateMaintenanceCmd
			assertNewMaint func(t *testing.T, maint *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance)
		}{
			{
				name:           "no changes",
				cmd:            &entity.UpdateMaintenanceCmd{},
				assertNewMaint: func(_ *testing.T, _ *entity.Maintenance, _ *entity.UpdateMaintenanceCmd, _ *entity.Maintenance) {},
			}, {
				name: "Title",
				cmd: &entity.UpdateMaintenanceCmd{
					Title: lo.ToPtr("new Title" + t.Name()),
				},
				assertNewMaint: func(t *testing.T, _ *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					t.Helper()
					require.Equal(t, lo.FromPtr(cmd.Title), newMaint.Title)
				},
			}, {
				name: "Description",
				cmd: &entity.UpdateMaintenanceCmd{
					Description: lo.ToPtr("new Description" + t.Name()),
				},
				assertNewMaint: func(t *testing.T, _ *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					t.Helper()
					require.Equal(t, lo.FromPtr(cmd.Description), newMaint.Description)
				},
			}, {
				name: "PlannedPeriod",
				cmd: &entity.UpdateMaintenanceCmd{
					PlannedStart: lo.ToPtr(now.Add(123 * time.Hour)),
				},
				assertNewMaint: func(t *testing.T, _ *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					t.Helper()
					newStart := lo.FromPtr(cmd.PlannedStart)
					newPlanned := entity.NewPeriod(newStart, newStart.Add(defaultPeriod.Duration()))

					require.Equal(t, newPlanned, newMaint.PlannedPeriod)
				},
			}, {
				name: "Scope",
				cmd: &entity.UpdateMaintenanceCmd{
					Scope: lo.ToPtr(entity.MaintenanceScopeResources),
				},
				assertNewMaint: func(t *testing.T, _ *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					t.Helper()
					require.Equal(t, lo.FromPtr(cmd.Scope), newMaint.Scope)
				},
			}, {
				name: "Impact",
				cmd: &entity.UpdateMaintenanceCmd{
					Impact: lo.ToPtr(entity.MaintenanceImpactNone),
				},
				assertNewMaint: func(t *testing.T, _ *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					t.Helper()
					require.Equal(t, lo.FromPtr(cmd.Impact), newMaint.Impact)
				},
			}, {
				name: "Resources",
				cmd: &entity.UpdateMaintenanceCmd{
					Resources: []uuid.UUID{testdbutils.MakeResource(ctx, t, service.resourcesStore).ID},
				},
				assertNewMaint: func(t *testing.T, _ *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					t.Helper()
					require.Equal(t, cmd.Resources, newMaint.Resources)
				},
			}, {
				name: "Resources[scope global]",
				cmd: &entity.UpdateMaintenanceCmd{
					Resources: []uuid.UUID{testdbutils.MakeResource(ctx, t, service.resourcesStore).ID},
					Scope:     lo.ToPtr(entity.MaintenanceScopeGlobal),
				},
				assertNewMaint: func(t *testing.T, _ *entity.Maintenance, _ *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					t.Helper()
					require.Len(t, newMaint.Resources, 0)
				},
			}, {
				name: "steps",
				cmd: &entity.UpdateMaintenanceCmd{
					Steps: []*entity.MaintenanceStepInput{{
						Order:               1,
						Description:         "new Description",
						RollbackDescription: "new RollbackDescription",
						DurationMinutes:     100,
					}},
				},
				assertNewMaint: func(t *testing.T, maint *entity.Maintenance, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					t.Helper()
					expSteps := lo.Map(cmd.Steps, func(item *entity.MaintenanceStepInput, i int) *entity.MaintenanceStep {
						return &entity.MaintenanceStep{
							ID:                  newMaint.Steps[i].ID,
							Order:               item.Order,
							Description:         item.Description,
							RollbackDescription: item.RollbackDescription,
							DurationMinutes:     item.DurationMinutes,
							Status:              entity.MaintenanceStepStatusPlanned,
							CreatedAt:           newMaint.Steps[i].CreatedAt,
						}
					})
					require.Equal(t, expSteps, newMaint.Steps)

					expPlannedPeriod := recalculatePlannedPeriod(maint.PlannedPeriod.Start, newMaint.Steps)
					require.Equal(t, expPlannedPeriod, newMaint.PlannedPeriod)
				},
			}, {
				name: "NotifyTargets",
				cmd: &entity.UpdateMaintenanceCmd{
					NotifyTargets: []*entity.NotifyTargetInput{{
						ChannelID: notifyChannel.ID,
					}},
				},
				assertNewMaint: func(t *testing.T, _ *entity.Maintenance, _ *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					t.Helper()
					require.Len(t, newMaint.NotifyTargets, 1)
					target := newMaint.NotifyTargets[0]
					require.Equal(t, notifyChannel.ID, target.ChannelID)
					require.Equal(t, notifyChannel.Transport, target.Transport)
					require.Equal(t, notifyChannel.TransportChannelID, target.TransportChannelID)
					require.Equal(t, notifyChannel.Name, target.ChannelName)
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore, defaultPeriod)
				tc.cmd.MaintID = maint.ID

				err := service.UpdateMaint(ctx, tc.cmd)
				require.NoError(t, err)

				newMaint, err := service.GetMaint(ctx, maint.ID)
				require.NoError(t, err)
				tc.assertNewMaint(t, maint, tc.cmd, newMaint)

				if len(tc.cmd.Steps) == 0 {
					require.Equal(t, maint.Steps, newMaint.Steps)
				}
				if len(tc.cmd.Resources) == 0 && lo.FromPtr(tc.cmd.Scope) != entity.MaintenanceScopeGlobal {
					require.ElementsMatch(t, maint.Resources, newMaint.Resources)
				}
			})
		}
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		t.Run("validation", func(t *testing.T) {
			t.Parallel()

			emptyStr := ""

			for _, tc := range []struct {
				name        string
				cmd         *entity.UpdateMaintenanceCmd
				expectedErr string
			}{
				{
					name:        "empty Title",
					cmd:         &entity.UpdateMaintenanceCmd{Title: &emptyStr},
					expectedErr: "Title: cannot be blank.",
				}, {
					name:        "empty Description",
					cmd:         &entity.UpdateMaintenanceCmd{Description: &emptyStr},
					expectedErr: "Description: cannot be blank.",
				}, {
					name:        "PlannedStart",
					cmd:         &entity.UpdateMaintenanceCmd{PlannedStart: lo.ToPtr(time.Time{})},
					expectedErr: "PlannedStart: cannot be blank.",
				}, {
					name:        "Scope",
					cmd:         &entity.UpdateMaintenanceCmd{Scope: lo.ToPtr(entity.MaintenanceScope(emptyStr))},
					expectedErr: "Scope: cannot be blank.",
				}, {
					name:        "Impact",
					cmd:         &entity.UpdateMaintenanceCmd{Impact: lo.ToPtr(entity.MaintenanceImpact(emptyStr))},
					expectedErr: "Impact: cannot be blank.",
				}, {
					name: "invalid Resources",
					cmd: &entity.UpdateMaintenanceCmd{
						Resources: []uuid.UUID{uuid.Nil},
					},
					expectedErr: "Resources: (0: cannot be blank.).",
				}, {
					name: "invalid steps",
					cmd: &entity.UpdateMaintenanceCmd{
						Steps: []*entity.MaintenanceStepInput{{
							Order:               0,
							Description:         "new Description",
							RollbackDescription: "new RollbackDescription",
							DurationMinutes:     100,
						}},
					},
					expectedErr: "Steps: (0: (Order: cannot be blank.).).",
				}, {
					// Proves the per-element rule really runs through the
					// pointer: if Each silently skipped it, a zero fire_at
					// would reach the store and fire immediately.
					name: "deferred notification without fire_at",
					cmd: &entity.UpdateMaintenanceCmd{
						DeferredNotifications: lo.ToPtr([]*entity.DeferredNotificationInput{
							{FireAt: time.Time{}},
						}),
					},
					expectedErr: "0: (FireAt: cannot be blank.).",
				}, {
					// Proves the cap survived moving Each out of the field rules.
					name: "too many deferred notifications",
					cmd: &entity.UpdateMaintenanceCmd{
						DeferredNotifications: lo.ToPtr(lo.RepeatBy(maxDeferredNotifications+1,
							func(_ int) *entity.DeferredNotificationInput {
								return &entity.DeferredNotificationInput{FireAt: time.Now().Add(time.Hour)}
							})),
					},
					expectedErr: "DeferredNotifications: the length must be no more than 10.",
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					maint := testdbutils.MakeMaint(ctx, t, service.maintStore, service.resourcesStore, defaultPeriod)
					tc.cmd.MaintID = maint.ID

					err := service.UpdateMaint(ctx, tc.cmd)
					require.EqualError(t, err, tc.expectedErr)
				})
			}
		})
	})
}

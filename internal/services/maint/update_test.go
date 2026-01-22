package maint

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	testdbutils "github.com/ruko1202/maintmode/test/utils/db"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"

	"github.com/ruko1202/maintmode/internal/entity"
)

//nolint:thelper
func TestUpdateDraft(t *testing.T) {
	t.Parallel()
	now := xtime.UTCNow()

	ctx := context.Background()
	s := initService(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name           string
			cmd            *entity.UpdateMaintenanceCmd
			assertNewMaint func(t *testing.T, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance)
		}{
			{
				name:           "no changes",
				cmd:            &entity.UpdateMaintenanceCmd{},
				assertNewMaint: func(_ *testing.T, _ *entity.UpdateMaintenanceCmd, _ *entity.Maintenance) {},
			}, {
				name: "Title",
				cmd: &entity.UpdateMaintenanceCmd{
					Title: lo.ToPtr("new Title" + t.Name()),
				},
				assertNewMaint: func(t *testing.T, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					require.Equal(t, lo.FromPtr(cmd.Title), newMaint.Title)
				},
			}, {
				name: "Description",
				cmd: &entity.UpdateMaintenanceCmd{
					Description: lo.ToPtr("new Description" + t.Name()),
				},
				assertNewMaint: func(t *testing.T, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					require.Equal(t, lo.FromPtr(cmd.Description), newMaint.Description)
				},
			}, {
				name: "PlannedPeriod",
				cmd: &entity.UpdateMaintenanceCmd{
					PlannedPeriod: lo.ToPtr(entity.NewPeriod(now.Add(123*time.Hour), now.Add(1234*time.Hour))),
				},
				assertNewMaint: func(t *testing.T, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					require.Equal(t, lo.FromPtr(cmd.PlannedPeriod), newMaint.PlannedPeriod)
				},
			}, {
				name: "Scope",
				cmd: &entity.UpdateMaintenanceCmd{
					Scope: lo.ToPtr(entity.MaintenanceScopeResources),
				},
				assertNewMaint: func(t *testing.T, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					require.Equal(t, lo.FromPtr(cmd.Scope), newMaint.Scope)
				},
			}, {
				name: "Impact",
				cmd: &entity.UpdateMaintenanceCmd{
					Impact: lo.ToPtr(entity.MaintenanceImpactNone),
				},
				assertNewMaint: func(t *testing.T, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					require.Equal(t, lo.FromPtr(cmd.Impact), newMaint.Impact)
				},
			}, {
				name: "Resources",
				cmd: &entity.UpdateMaintenanceCmd{
					Resources: []*entity.Resource{{
						ID:   xuuid.New(),
						Type: entity.ResourceTypeCluster,
					}},
				},
				assertNewMaint: func(t *testing.T, cmd *entity.UpdateMaintenanceCmd, newMaint *entity.Maintenance) {
					require.Equal(t, cmd.Resources, newMaint.Resources)
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				maint := testdbutils.MakeMaint(ctx, t, s.maintStore, entity.NewPeriod(now, now.Add(time.Hour)))
				tc.cmd.MaintID = maint.ID

				err := s.Update(ctx, tc.cmd)
				require.NoError(t, err)

				newMaint, err := s.Get(ctx, maint.ID)
				require.NoError(t, err)
				tc.assertNewMaint(t, tc.cmd, newMaint)
			})
		}
	})
}

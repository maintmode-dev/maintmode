package testdbutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

type MaintChanger func(m *entity.Maintenance)

func WithScope(scope entity.MaintenanceScope) MaintChanger {
	return func(m *entity.Maintenance) {
		m.Scope = scope
		if scope == entity.MaintenanceScopeGlobal {
			m.Resources = nil
		}
	}
}

func WithResources(resources ...*entity.Resource) MaintChanger {
	return func(m *entity.Maintenance) {
		m.Resources = resources
	}
}

func WithStatus(status entity.MaintenanceStatus) MaintChanger {
	return func(m *entity.Maintenance) {
		m.Status = status
	}
}

func WithSteps(step []*entity.MaintenanceStep) MaintChanger {
	return func(m *entity.Maintenance) {
		m.Steps = step
	}
}

func MakeMaint(ctx context.Context, t *testing.T, store *maintenances.Store, period entity.Period, changers ...MaintChanger) *entity.Maintenance {
	t.Helper()

	maint := &entity.Maintenance{
		ID:            xuuid.New(),
		Title:         "Title" + t.Name(),
		Description:   "Description" + t.Name(),
		PlannedPeriod: period,
		Scope:         entity.MaintenanceScopeResources,
		Status:        entity.MaintenanceStatusDraft,
		Impact:        entity.MaintenanceImpactFull,
		CreatedAt:     xtime.UTCNow(),
		Resources: []*entity.Resource{
			{
				ID:   xuuid.New(),
				Type: entity.ResourceTypeService,
			}, {
				ID:   xuuid.New(),
				Type: entity.ResourceTypeDatabase,
			},
		},
		Steps: []*entity.MaintenanceStep{{
			Order:               1,
			Description:         "Step 1" + t.Name(),
			RollbackDescription: "Rollback Step 1" + t.Name(),
			DurationMinutes:     1,
			Status:              entity.MaintenanceStepStatusPlanned,
		}},
	}
	for _, changer := range changers {
		changer(maint)
	}

	created, err := store.CreateMaint(ctx, maint)
	require.NoError(t, err)

	if len(maint.Resources) > 0 {
		err = store.AddResources(ctx, created.ID, maint.Resources)
		require.NoError(t, err)
		created.Resources = maint.Resources
	}
	if len(maint.Steps) > 0 {
		steps, err := store.AddSteps(ctx, created.ID, maint.Steps)
		require.NoError(t, err)
		created.Steps = steps
	}

	return created
}

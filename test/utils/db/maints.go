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
	}
	for _, changer := range changers {
		changer(maint)
	}

	err := store.Create(ctx, maint)
	require.NoError(t, err)

	err = store.AddResources(ctx, maint.ID, maint.Resources)
	require.NoError(t, err)

	return maint
}

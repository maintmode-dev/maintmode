package testdbutils

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/maintenances"
	"github.com/ruko1202/maintmode/internal/storages/resources"
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

func WithResources(ids ...uuid.UUID) MaintChanger {
	return func(m *entity.Maintenance) {
		m.Resources = ids
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

func MakeResource(ctx context.Context, t *testing.T, store *resources.Store) *entity.ResourceDetails {
	t.Helper()

	resource, err := store.Create(ctx, &entity.ResourceDetails{
		Name:        "Resource" + xuuid.NewString(),
		Description: "Description" + t.Name(),
	})
	require.NoError(t, err)

	return resource
}

func MakeResources(ctx context.Context, t *testing.T, store *resources.Store, count int) []uuid.UUID {
	t.Helper()

	result := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, MakeResource(ctx, t, store).ID)
	}

	return result
}

func MakeMaint(
	ctx context.Context,
	t *testing.T,
	maintStore *maintenances.Store,
	resourceStore *resources.Store,
	period entity.Period,
	changers ...MaintChanger,
) *entity.Maintenance {
	t.Helper()

	maint := &entity.Maintenance{
		ID:             xuuid.New(),
		Title:          "Title" + t.Name(),
		Description:    "Description" + t.Name(),
		PlannedPeriod:  period,
		Scope:          entity.MaintenanceScopeResources,
		Status:         entity.MaintenanceStatusDraft,
		Impact:         entity.MaintenanceImpactFull,
		CreatedAt:      xtime.UTCNow(),
		ApproverUserID: xuuid.New(),
		Resources:      MakeResources(ctx, t, resourceStore, 2),
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

	created, err := maintStore.CreateMaint(ctx, maint)
	require.NoError(t, err)

	if len(maint.Resources) > 0 {
		err = maintStore.AddResources(ctx, created.ID, maint.Resources)
		require.NoError(t, err)
		created.Resources = maint.Resources
	}
	if len(maint.Steps) > 0 {
		steps, err := maintStore.AddSteps(ctx, created.ID, maint.Steps)
		require.NoError(t, err)
		created.Steps = steps
	}

	return created
}

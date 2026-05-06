package maintenances

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	testdbconnutils "github.com/ruko1202/maintmode/test/utils/db/conn"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/resources"

	"github.com/ruko1202/maintmode/internal/utils/closer"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

var (
	db             *sqlx.DB
	resourcesStore *resources.Store
)

func TestMain(m *testing.M) {
	db = testdbconnutils.NewDB(config.LoadAppConfig())
	closer.Add(db.Close)

	resourcesStore = resources.NewStore(db)

	code := m.Run()

	os.Exit(code)
}

func makeMaint(ctx context.Context, t *testing.T, store *Store, period entity.Period) *entity.Maintenance {
	t.Helper()

	maint := &entity.Maintenance{
		Title:         "Title" + t.Name(),
		Description:   "Description" + t.Name(),
		PlannedPeriod: period,
		Scope:         entity.MaintenanceScopeResources,
		Status:        entity.MaintenanceStatusPlanned,
		Impact:        entity.MaintenanceImpactFull,
		Resources: []*entity.Resource{
			makeResource(ctx, t, entity.ResourceTypeService),
			makeResource(ctx, t, entity.ResourceTypeDatabase),
		},
	}

	created, err := store.CreateMaint(ctx, maint)
	require.NoError(t, err)

	err = store.AddResources(ctx, created.ID, maint.Resources)
	require.NoError(t, err)
	created.Resources = maint.Resources

	return created
}

func makeResource(ctx context.Context, t *testing.T, resourceType entity.ResourceType) *entity.Resource {
	t.Helper()

	resource, err := resourcesStore.Create(ctx, &entity.ResourceDetails{
		Name:        "Resource" + xuuid.NewString(),
		Description: "Description" + t.Name(),
	})
	require.NoError(t, err)

	return &entity.Resource{
		ID:   resource.ID,
		Type: resourceType,
	}
}

func makeSteps(ctx context.Context, t *testing.T, store *Store, maintID uuid.UUID) []*entity.MaintenanceStep {
	t.Helper()

	steps, err := store.AddSteps(ctx, maintID, []*entity.MaintenanceStep{
		{
			Order:               1,
			Description:         "Description1",
			RollbackDescription: "RollbackDescription1",
			DurationMinutes:     1,
			Status:              entity.MaintenanceStepStatusPlanned,
		}, {
			Order:               2,
			Description:         "Description2",
			RollbackDescription: "RollbackDescription2",
			DurationMinutes:     2,
			Status:              entity.MaintenanceStepStatusPlanned,
		},
	})
	require.NoError(t, err)

	return steps
}

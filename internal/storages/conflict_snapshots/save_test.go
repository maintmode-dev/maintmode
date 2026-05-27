package conflictsnapshots

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

func TestSave(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	t.Run("save single conflict snapshot", func(t *testing.T) {
		t.Parallel()

		now := xtime.UTCNow().Round(time.Microsecond)
		conflictPeriod := entity.Period{
			Start: now.Add(time.Hour),
			End:   lo.ToPtr(now.Add(2 * time.Hour)),
		}

		resource1 := makeResource(ctx, t)
		resource2 := makeResource(ctx, t)

		maintenance := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, conflictPeriod,
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithResources(resource1.ID),
		)

		conflictedMaintenance := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, conflictPeriod,
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithResources(resource2.ID),
		)

		snapshots := []*entity.ConflictWithResources{
			{
				Conflict: &entity.Conflict{
					MaintenanceID: conflictedMaintenance.ID,
					Title:         "Test Conflict",
					OverlapStart:  conflictPeriod.Start,
					OverlapEnd:    *conflictPeriod.End,
					Scope:         entity.MaintenanceScopeResources,
				},
				Resources: []uuid.UUID{resource2.ID},
			},
		}

		err := store.Save(ctx, maintenance.ID, snapshots)
		require.NoError(t, err)

		retrieved, err := store.GetSnapshots(ctx, maintenance.ID)
		require.NoError(t, err)
		require.Len(t, retrieved, 1)

		actual := retrieved[0]
		require.Equal(t, conflictedMaintenance.ID, actual.MaintenanceID)
		require.Equal(t, conflictPeriod.Start, actual.OverlapStart)
		require.Equal(t, *conflictPeriod.End, actual.OverlapEnd)
		require.Len(t, actual.Resources, 1)
		require.Equal(t, resource2.ID, actual.Resources[0])
	})

	t.Run("save multiple conflict snapshots", func(t *testing.T) {
		t.Parallel()

		now := xtime.UTCNow().Round(time.Microsecond)

		resource1 := makeResource(ctx, t)
		resource2 := makeResource(ctx, t)
		resource3 := makeResource(ctx, t)

		maintenance := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, entity.NewPeriod(now.Add(time.Hour), now.Add(5*time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithResources(resource1.ID),
		)

		conflictedMaint1 := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, entity.NewPeriod(now.Add(time.Hour), now.Add(2*time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithResources(resource2.ID),
		)

		conflictedMaint2 := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, entity.NewPeriod(now.Add(3*time.Hour), now.Add(4*time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithScope(entity.MaintenanceScopeGlobal),
			testdbutils.WithResources(resource3.ID),
		)

		snapshots := []*entity.ConflictWithResources{
			{
				Conflict: &entity.Conflict{
					MaintenanceID: conflictedMaint1.ID,
					Title:         "Conflict 1",
					OverlapStart:  now.Add(time.Hour),
					OverlapEnd:    now.Add(2 * time.Hour),
					Scope:         entity.MaintenanceScopeResources,
				},
				Resources: []uuid.UUID{conflictedMaint1.Resources[0]},
			},
			{
				Conflict: &entity.Conflict{
					MaintenanceID: conflictedMaint2.ID,
					Title:         "Conflict 2",
					OverlapStart:  now.Add(3 * time.Hour),
					OverlapEnd:    now.Add(4 * time.Hour),
					Scope:         entity.MaintenanceScopeGlobal,
				},
				Resources: nil,
			},
		}

		err := store.Save(ctx, maintenance.ID, snapshots)
		require.NoError(t, err)

		retrieved, err := store.GetSnapshots(ctx, maintenance.ID)
		require.NoError(t, err)
		require.Len(t, retrieved, 2)
	})

	t.Run("save empty snapshots returns nil error", func(t *testing.T) {
		t.Parallel()

		maintenanceID := xuuid.New()
		err := store.Save(ctx, maintenanceID, []*entity.ConflictWithResources{})
		require.NoError(t, err)
	})

	t.Run("save nil snapshots returns nil error", func(t *testing.T) {
		t.Parallel()

		maintenanceID := xuuid.New()
		err := store.Save(ctx, maintenanceID, nil)
		require.NoError(t, err)
	})
}

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
	testtimeutils "github.com/ruko1202/maintmode/test/utils/time"
)

func TestGetSnapshots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	t.Run("get existing snapshots", func(t *testing.T) {
		t.Parallel()

		now := xtime.UTCNow()

		// Create resources
		resource1 := makeResource(ctx, t)
		resource2 := makeResource(ctx, t)
		resource3 := makeResource(ctx, t)

		// Create maintenance
		maintenance := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, entity.NewPeriod(now.Add(time.Hour), now.Add(5*time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithResources(resource1.ID),
		)

		// Create conflicted maintenances
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
					Title:         "Conflict A",
					OverlapStart:  now.Add(time.Hour),
					OverlapEnd:    now.Add(2 * time.Hour),
					Scope:         entity.MaintenanceScopeResources,
				},
				Resources: []uuid.UUID{conflictedMaint1.Resources[0]},
			},
			{
				Conflict: &entity.Conflict{
					MaintenanceID: conflictedMaint2.ID,
					Title:         "Conflict B",
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

		// Create map for easier verification
		retrievedMap := lo.SliceToMap(retrieved, func(item *entity.ConflictWithResources) (uuid.UUID, *entity.ConflictWithResources) {
			return item.MaintenanceID, item
		})

		// Verify first snapshot
		actual1, ok := retrievedMap[conflictedMaint1.ID]
		require.True(t, ok)
		require.Equal(t, entity.MaintenanceScopeResources, actual1.Scope)
		require.Equal(t, conflictedMaint1.Title, actual1.Title)
		require.Equal(t, testtimeutils.OverlapStart(conflictedMaint1.PlannedPeriod, maintenance.PlannedPeriod), actual1.OverlapStart)
		require.Equal(t, testtimeutils.OverlapEnd(conflictedMaint1.PlannedPeriod, maintenance.PlannedPeriod), actual1.OverlapEnd)
		require.Len(t, actual1.Resources, 1)

		// Verify second snapshot
		actual2, ok := retrievedMap[conflictedMaint2.ID]
		require.True(t, ok)
		require.Equal(t, entity.MaintenanceScopeGlobal, actual2.Scope)
		require.Equal(t, conflictedMaint2.Title, actual2.Title)
		require.Equal(t, testtimeutils.OverlapStart(conflictedMaint2.PlannedPeriod, maintenance.PlannedPeriod), actual2.OverlapStart)
		require.Equal(t, testtimeutils.OverlapEnd(conflictedMaint2.PlannedPeriod, maintenance.PlannedPeriod), actual2.OverlapEnd)
		require.Empty(t, actual2.Resources)
	})

	t.Run("get snapshots for non-existent maintenance", func(t *testing.T) {
		t.Parallel()

		nonExistentID := xuuid.New()

		retrieved, err := store.GetSnapshots(ctx, nonExistentID)
		require.NoError(t, err)
		require.Empty(t, retrieved)
	})

	t.Run("get snapshots after multiple saves", func(t *testing.T) {
		t.Parallel()

		now := xtime.UTCNow()

		// Create resources
		resource1 := makeResource(ctx, t)
		resource2 := makeResource(ctx, t)
		resource3 := makeResource(ctx, t)

		// Create maintenance
		maintenance := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, entity.NewPeriod(now.Add(time.Hour), now.Add(5*time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithResources(resource1.ID),
		)

		// Create conflicted maintenances
		conflictedMaint1 := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, entity.NewPeriod(now.Add(time.Hour), now.Add(2*time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithResources(resource2.ID),
		)

		conflictedMaint2 := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore, entity.NewPeriod(now.Add(3*time.Hour), now.Add(4*time.Hour)),
			testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
			testdbutils.WithResources(resource3.ID),
		)

		// First save
		firstSnapshots := []*entity.ConflictWithResources{
			{
				Conflict: &entity.Conflict{
					MaintenanceID: conflictedMaint1.ID,
					Title:         "First Conflict",
					OverlapStart:  now.Add(time.Hour),
					OverlapEnd:    now.Add(2 * time.Hour),
					Scope:         entity.MaintenanceScopeResources,
				},
				Resources: []uuid.UUID{conflictedMaint1.Resources[0]},
			},
		}

		err := store.Save(ctx, maintenance.ID, firstSnapshots)
		require.NoError(t, err)

		// Second save - adds more snapshots
		secondSnapshots := []*entity.ConflictWithResources{
			{
				Conflict: &entity.Conflict{
					MaintenanceID: conflictedMaint2.ID,
					Title:         "Second Conflict",
					OverlapStart:  now.Add(3 * time.Hour),
					OverlapEnd:    now.Add(4 * time.Hour),
					Scope:         entity.MaintenanceScopeResources,
				},
				Resources: []uuid.UUID{conflictedMaint2.Resources[0]},
			},
		}

		err = store.Save(ctx, maintenance.ID, secondSnapshots)
		require.NoError(t, err)

		// Retrieve all snapshots
		retrieved, err := store.GetSnapshots(ctx, maintenance.ID)
		require.NoError(t, err)
		require.Len(t, retrieved, 2)
	})
}

// TestGetSnapshots_DoesNotMutate pins the audit invariant: reading a snapshot
// never rewrites it. The rows are the record of what the approver accepted, and
// a read path that "repairs" or re-times them would quietly destroy that.
func TestGetSnapshots_DoesNotMutate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	now := xtime.UTCNow()

	resource := makeResource(ctx, t)
	maintenance := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
		entity.NewPeriod(now.Add(time.Hour), now.Add(5*time.Hour)),
		testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		testdbutils.WithResources(resource.ID),
	)

	conflicted := testdbutils.MakeMaint(ctx, t, maintsStore, resourcesStore,
		entity.NewPeriod(now.Add(time.Hour), now.Add(2*time.Hour)),
		testdbutils.WithStatus(entity.MaintenanceStatusPlanned),
		testdbutils.WithResources(resource.ID),
	)

	require.NoError(t, store.Save(ctx, maintenance.ID, []*entity.ConflictWithResources{
		{
			Conflict: &entity.Conflict{
				MaintenanceID: conflicted.ID,
				Title:         "frozen at approval",
				OverlapStart:  now.Add(time.Hour),
				OverlapEnd:    now.Add(2 * time.Hour),
				Scope:         entity.MaintenanceScopeResources,
			},
			Resources: []uuid.UUID{resource.ID},
		},
	}))

	type row struct {
		ID                      uuid.UUID  `db:"id"`
		MaintenanceID           uuid.UUID  `db:"maintenance_id"`
		ConflictedMaintenanceID uuid.UUID  `db:"conflicted_maintenance_id"`
		ResourceID              *uuid.UUID `db:"resource_id"`
		CreatedAt               time.Time  `db:"created_at"`
	}

	const q = `SELECT id, maintenance_id, conflicted_maintenance_id, resource_id, created_at
		FROM maintenance_conflict_snapshot WHERE maintenance_id = $1 ORDER BY id`

	before := make([]row, 0)
	require.NoError(t, db.SelectContext(ctx, &before, q, maintenance.ID))
	require.NotEmpty(t, before, "the fixture must have written rows to compare")

	for range 3 {
		_, err := store.GetSnapshots(ctx, maintenance.ID)
		require.NoError(t, err)
	}

	after := make([]row, 0)
	require.NoError(t, db.SelectContext(ctx, &after, q, maintenance.ID))

	require.Equal(t, before, after, "reading a snapshot must not write to it")
}

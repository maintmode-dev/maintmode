package maintenances

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestCreatedByUserID covers the author column round-trip: a draft created with
// an author persists and reads back the id, an update does not drop the author,
// and the NOT NULL constraint rejects an authorless row.
func TestCreatedByUserID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	now := xtime.UTCNow()
	period := entity.NewPeriod(now, now.Add(time.Minute))

	t.Run("persists and reads back author", func(t *testing.T) {
		t.Parallel()

		authorID := xuuid.New()

		created, err := store.CreateMaint(ctx, &entity.Maintenance{
			Title:           "Title" + t.Name(),
			Description:     "Description" + t.Name(),
			PlannedPeriod:   period,
			Scope:           entity.MaintenanceScopeGlobal,
			Status:          entity.MaintenanceStatusDraft,
			Impact:          entity.MaintenanceImpactFull,
			CreatedByUserID: authorID,
		})
		require.NoError(t, err)
		require.Equal(t, authorID, created.CreatedByUserID)

		got, err := store.GetMaint(ctx, created.ID)
		require.NoError(t, err)
		require.Equal(t, authorID, got.CreatedByUserID)
	})

	t.Run("update preserves author", func(t *testing.T) {
		t.Parallel()

		authorID := xuuid.New()

		created, err := store.CreateMaint(ctx, &entity.Maintenance{
			Title:           "Title" + t.Name(),
			Description:     "Description" + t.Name(),
			PlannedPeriod:   period,
			Scope:           entity.MaintenanceScopeGlobal,
			Status:          entity.MaintenanceStatusDraft,
			Impact:          entity.MaintenanceImpactFull,
			CreatedByUserID: authorID,
		})
		require.NoError(t, err)

		// A read-modify-write cycle must not drop the author: the entity carries
		// CreatedByUserID through the mapper, so UpdateMaint rewrites it as-is.
		created.Description += " updated"
		require.NoError(t, store.UpdateMaint(ctx, created))

		got, err := store.GetMaint(ctx, created.ID)
		require.NoError(t, err)
		require.Equal(t, authorID, got.CreatedByUserID)
	})
}

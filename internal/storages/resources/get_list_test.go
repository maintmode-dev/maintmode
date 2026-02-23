package resources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestStore_GetResourcesLikeName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	t.Run("successful search with matching name", func(t *testing.T) {
		t.Parallel()
		resource := makeResource(ctx, t, store)

		results, err := store.GetResourcesLikeName(ctx, resource.Name)
		require.NoError(t, err)
		require.Equal(t, 1, len(results))
		require.Equal(t, resource.ID, results[0].ID)
		require.Equal(t, resource.Name, results[0].Name)
	})

	t.Run("successful search with partial match", func(t *testing.T) {
		t.Parallel()
		resource := makeResource(ctx, t, store)

		results, err := store.GetResourcesLikeName(ctx, resource.Name[:len(resource.Name)/2])
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(results), 1)
	})

	t.Run("empty result when no matches", func(t *testing.T) {
		t.Parallel()
		results, err := store.GetResourcesLikeName(ctx, "nonexistent-resource-name")
		require.NoError(t, err)
		require.Equal(t, 0, len(results))
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()
		for range 101 {
			makeResource(ctx, t, store)
		}
		results, err := store.GetResourcesLikeName(ctx, "")
		require.NoError(t, err)
		require.Equal(t, resourceListLimit, len(results))
	})

	t.Run("multiple results for partial match", func(t *testing.T) {
		t.Parallel()
		baseName := "TestResource" + xtime.UTCNow().String()
		resources := []*entity.ResourceDetails{
			{
				ID:          xuuid.New(),
				Name:        baseName + "-one",
				Description: "Description one",
				ExternalID:  nil,
				CreatedAt:   xtime.UTCNow(),
			},
			{
				ID:          xuuid.New(),
				Name:        baseName + "-two",
				Description: "Description two",
				ExternalID:  nil,
				CreatedAt:   xtime.UTCNow(),
			},
		}

		for _, r := range resources {
			err := store.Create(ctx, r)
			require.NoError(t, err)
		}

		results, err := store.GetResourcesLikeName(ctx, baseName)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(results), 2)
	})
}

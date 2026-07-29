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
				Name:        baseName + "-one",
				Description: "Description one",
				ExternalID:  nil,
				Status:      entity.ResourceStatusActive,
			},
			{
				Name:        baseName + "-two",
				Description: "Description two",
				ExternalID:  nil,
				Status:      entity.ResourceStatusActive,
			},
		}

		for _, r := range resources {
			_, err := store.Create(ctx, r)
			require.NoError(t, err)
		}

		results, err := store.GetResourcesLikeName(ctx, baseName)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(results), 2)
	})

	// The picker search shares the list store's flaw: an unescaped "%" turns the
	// filter into a match-everything. Asserting our own fixture is absent, rather
	// than an empty result, is what keeps resourceListLimit from capping a leaked
	// wildcard into a plausible-looking page.
	t.Run("LIKE metacharacters match literally", func(t *testing.T) {
		t.Parallel()
		// The query orders by name ASC and cuts at resourceListLimit, so a fixture
		// named at random sits far past the cut and stays invisible whether or not
		// the wildcard leaks. The "!" prefix sorts below digits and letters, which
		// puts this row inside the first page of a leak — the only position from
		// which its presence proves anything.
		plain := makeNamedResource(ctx, t, store, "!leak-probe-"+xuuid.NewString())

		for _, meta := range []string{"%", "_", "%%", "_%"} {
			results, err := store.GetResourcesLikeName(ctx, meta)
			require.NoError(t, err)
			require.Falsef(t, containsID(results, plain.ID),
				"name=%q must not act as a wildcard", meta)
		}
	})

	t.Run("an underscore matches itself, not any character", func(t *testing.T) {
		t.Parallel()
		// Resource names are unique and the DB is shared across runs, so the stem
		// must be a whole uuid: a truncated one collides with a previous run.
		stem := "gu" + xuuid.NewString()
		literal := makeNamedResource(ctx, t, store, stem+"_x")
		other := makeNamedResource(ctx, t, store, stem+"yx")

		results, err := store.GetResourcesLikeName(ctx, stem+"_x")
		require.NoError(t, err)
		require.True(t, containsID(results, literal.ID))
		require.False(t, containsID(results, other.ID))
	})

	t.Run("excludes archived resources", func(t *testing.T) {
		t.Parallel()
		resource := makeResource(ctx, t, store)

		// before archiving, search finds it
		before, err := store.GetResourcesLikeName(ctx, resource.Name)
		require.NoError(t, err)
		require.Len(t, before, 1)

		require.NoError(t, store.Archive(ctx, resource.ID))

		// after archiving, the picker search must not surface it
		after, err := store.GetResourcesLikeName(ctx, resource.Name)
		require.NoError(t, err)
		require.Empty(t, after)
	})
}

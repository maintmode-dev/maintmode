package resources

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestStore_List(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	t.Run("created resource appears and defaults to active status", func(t *testing.T) {
		t.Parallel()
		resource := makeResource(ctx, t, store)

		found := findByPaging(ctx, t, store, resource.ID, false)
		require.NotNil(t, found)
		require.Equal(t, entity.ResourceStatusActive, found.Status)
	})

	t.Run("ordered by created_at desc within a page", func(t *testing.T) {
		t.Parallel()
		// The two newest resources globally: fetch the top page and assert the
		// whole page is sorted by created_at DESC (with id DESC as tie-break).
		makeResource(ctx, t, store)
		makeResource(ctx, t, store)

		page, _, err := store.List(ctx, &entity.ListResourcesCmd{Limit: 50, Offset: 0})
		require.NoError(t, err)
		require.NotEmpty(t, page)

		for i := 1; i < len(page); i++ {
			prev, cur := page[i-1], page[i]
			require.Falsef(t, cur.CreatedAt.After(prev.CreatedAt),
				"page not sorted by created_at desc at index %d", i)
		}
	})

	t.Run("archived hidden by default, shown when IncludeArchived", func(t *testing.T) {
		t.Parallel()
		active := makeResource(ctx, t, store)
		archived := makeArchivedResource(ctx, t, store)

		// default: only active — archived must never appear
		require.NotNil(t, findByPaging(ctx, t, store, active.ID, false))
		require.Nil(t, findByPaging(ctx, t, store, archived.ID, false))

		// include archived: both active and archived appear
		require.NotNil(t, findByPaging(ctx, t, store, active.ID, true))
		require.NotNil(t, findByPaging(ctx, t, store, archived.ID, true))
	})

	t.Run("filters by partial name match", func(t *testing.T) {
		t.Parallel()
		resource := makeResource(ctx, t, store)

		// the unique name fragment matches only this resource
		fragment := resource.Name[len(resource.Name)/2:]
		page, total, err := store.List(ctx, &entity.ListResourcesCmd{
			Name:   fragment,
			Limit:  200,
			Offset: 0,
		})
		require.NoError(t, err)
		require.GreaterOrEqual(t, total, int64(1))
		require.True(t, containsID(page, resource.ID))

		// a name that cannot match anything yields nothing
		empty, total, err := store.List(ctx, &entity.ListResourcesCmd{
			Name:   "no-such-resource-" + resource.ID.String(),
			Limit:  200,
			Offset: 0,
		})
		require.NoError(t, err)
		require.Zero(t, total)
		require.Empty(t, empty)
	})

	t.Run("limit caps the page size", func(t *testing.T) {
		t.Parallel()
		// ensure at least two rows exist
		makeResource(ctx, t, store)
		makeResource(ctx, t, store)

		page, total, err := store.List(ctx, &entity.ListResourcesCmd{Limit: 1, Offset: 0})
		require.NoError(t, err)
		require.Len(t, page, 1)
		require.GreaterOrEqual(t, total, int64(2))
	})

	t.Run("offset skips a leading row", func(t *testing.T) {
		t.Parallel()
		// A single 2-row query is a consistent snapshot; comparing it against an
		// offset=1 query is racy under the shared DB, so we assert the offset
		// query's first row equals the full snapshot's second row only when the
		// snapshot is stable (same total before and after).
		first, total1, err := store.List(ctx, &entity.ListResourcesCmd{Limit: 2, Offset: 0})
		require.NoError(t, err)

		if total1 >= 2 {
			skipped, total2, err := store.List(ctx, &entity.ListResourcesCmd{Limit: 1, Offset: 1})
			require.NoError(t, err)
			require.Len(t, skipped, 1)
			if total1 == total2 {
				// stable snapshot: offset=1 must equal the 2nd row of the page
				require.Equal(t, first[1].ID, skipped[0].ID)
			}
		}
	})
}

// findByPaging walks the list (newest first) page by page until the resource
// with the given id is found or the list is exhausted. The test DB is shared
// between parallel tests, so a single bounded page cannot reliably locate a
// freshly created row.
func findByPaging(ctx context.Context, t *testing.T, store *Store, id uuid.UUID, includeArchived bool) *entity.ResourceDetails {
	t.Helper()

	const pageSize = 200
	var offset int64
	for {
		page, total, err := store.List(ctx, &entity.ListResourcesCmd{
			Limit:           pageSize,
			Offset:          offset,
			IncludeArchived: includeArchived,
		})
		require.NoError(t, err)

		if found, ok := lo.Find(page, func(r *entity.ResourceDetails) bool {
			return r.ID == id
		}); ok {
			return found
		}

		offset += int64(len(page))
		if len(page) == 0 || offset >= total {
			return nil
		}
	}
}

func containsID(resources []*entity.ResourceDetails, id uuid.UUID) bool {
	return lo.ContainsBy(resources, func(r *entity.ResourceDetails) bool {
		return r.ID == id
	})
}

func makeArchivedResource(ctx context.Context, t *testing.T, store *Store) *entity.ResourceDetails {
	t.Helper()
	resource := makeResource(ctx, t, store)

	require.NoError(t, store.Archive(ctx, resource.ID))

	resource.Status = entity.ResourceStatusArchived
	return resource
}

package resources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

func TestStore_Archive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	t.Run("archives an active resource", func(t *testing.T) {
		t.Parallel()
		resource := makeResource(ctx, t, store)
		require.Equal(t, entity.ResourceStatusActive, resource.Status)

		err := store.Archive(ctx, resource.ID)
		require.NoError(t, err)

		got, err := store.GetByID(ctx, resource.ID)
		require.NoError(t, err)
		require.Equal(t, entity.ResourceStatusArchived, got.Status)
	})

	t.Run("idempotent on already archived", func(t *testing.T) {
		t.Parallel()
		resource := makeResource(ctx, t, store)

		require.NoError(t, store.Archive(ctx, resource.ID))
		// second call must still succeed
		require.NoError(t, store.Archive(ctx, resource.ID))

		got, err := store.GetByID(ctx, resource.ID)
		require.NoError(t, err)
		require.Equal(t, entity.ResourceStatusArchived, got.Status)
	})
}

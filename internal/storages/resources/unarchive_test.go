package resources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestStore_Unarchive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	t.Run("restores an archived resource to active", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(ctx, t, store)
		require.NoError(t, store.Archive(ctx, resource.ID))

		err := store.Unarchive(ctx, resource.ID)
		require.NoError(t, err)

		got, err := store.GetByID(ctx, resource.ID)
		require.NoError(t, err)
		require.Equal(t, entity.ResourceStatusActive, got.Status)
	})

	t.Run("idempotent on already active", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(ctx, t, store)

		require.NoError(t, store.Unarchive(ctx, resource.ID))
		require.NoError(t, store.Unarchive(ctx, resource.ID))

		got, err := store.GetByID(ctx, resource.ID)
		require.NoError(t, err)
		require.Equal(t, entity.ResourceStatusActive, got.Status)
	})

	t.Run("unknown id is a no-op success", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, store.Unarchive(ctx, xuuid.New()))
	})
}

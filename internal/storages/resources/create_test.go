package resources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/pkg/generated/postgres/public/model"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		resource := &model.Resources{
			Name:        "Name" + t.Name(),
			Description: "Description" + t.Name(),
		}

		err := store.Create(ctx, resource)
		require.NoError(t, err)
		require.NotNil(t, resource)

		dbResource, err := store.GetByID(ctx, resource.ID)
		require.NoError(t, err)
		require.NotNil(t, dbResource)
		require.Equal(t, resource, dbResource)
	})
}

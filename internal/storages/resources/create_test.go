package resources

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		resource := &entity.ResourceDetails{
			Name:        "Name" + t.Name() + "-" + xuuid.NewString(),
			Description: "Description" + t.Name(),
			ExternalID:  lo.ToPtr(xuuid.NewString()),
			Status:      entity.ResourceStatusActive,
		}

		created, err := store.Create(ctx, resource)
		require.NoError(t, err)
		require.NotNil(t, created)

		resource.ID = created.ID
		resource.CreatedAt = created.CreatedAt
		require.Equal(t, resource, created)

		dbResource, err := store.GetByID(ctx, created.ID)
		require.NoError(t, err)
		require.NotNil(t, dbResource)
		require.Equal(t, created, dbResource)
	})

	t.Run("duplicate", func(t *testing.T) {
		t.Parallel()

		resource := &entity.ResourceDetails{
			Name:        "Name" + t.Name() + "-" + xuuid.NewString(),
			Description: "Description" + t.Name(),
			ExternalID:  lo.ToPtr(xuuid.NewString()),
		}

		_, err := store.Create(ctx, resource)
		require.NoError(t, err)

		_, err = store.Create(ctx, resource)
		require.EqualError(t, err, apperr.ErrResourceAlreadyExists.Error())
	})
}

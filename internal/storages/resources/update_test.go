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

func TestStore_Update(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(db)

	t.Run("updates fields and preserves status and created_at", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(ctx, t, store)

		resource.Name = "Renamed" + t.Name() + xuuid.NewString()
		resource.Description = "Updated description"
		resource.ExternalID = lo.ToPtr("ext-" + xuuid.NewString())

		updated, err := store.Update(ctx, resource)
		require.NoError(t, err)
		require.NotNil(t, updated)
		require.Equal(t, resource.Name, updated.Name)
		require.Equal(t, resource.Description, updated.Description)
		require.Equal(t, resource.ExternalID, updated.ExternalID)
		require.Equal(t, entity.ResourceStatusActive, updated.Status)
		require.Equal(t, resource.CreatedAt, updated.CreatedAt)
		require.NotNil(t, updated.UpdatedAt)

		got, err := store.GetByID(ctx, resource.ID)
		require.NoError(t, err)
		require.Equal(t, updated, got)
	})

	t.Run("clears external_id when set to empty string", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(ctx, t, store)
		resource.ExternalID = lo.ToPtr("")

		updated, err := store.Update(ctx, resource)
		require.NoError(t, err)
		require.Equal(t, lo.ToPtr(""), updated.ExternalID)
	})

	t.Run("rename collision returns ErrResourceAlreadyExists", func(t *testing.T) {
		t.Parallel()

		existing := makeResource(ctx, t, store)
		other := makeResource(ctx, t, store)

		other.Name = existing.Name

		_, err := store.Update(ctx, other)
		require.EqualError(t, err, apperr.ErrResourceAlreadyExists.Error())
	})

	t.Run("unknown id returns ErrResourceNotFound", func(t *testing.T) {
		t.Parallel()

		resource := &entity.ResourceDetails{
			ID:          xuuid.New(),
			Name:        "Name" + t.Name() + xuuid.NewString(),
			Description: "Description" + t.Name(),
			Status:      entity.ResourceStatusActive,
		}

		_, err := store.Update(ctx, resource)
		require.EqualError(t, err, apperr.ErrResourceNotFound.Error())
	})
}

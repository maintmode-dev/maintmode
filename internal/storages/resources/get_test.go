package resources

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/pkg/generated/maintmode/public/model"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		resource := makeResource(ctx, t, store)

		t.Run("GetByID", func(t *testing.T) {
			t.Parallel()

			dbResource, err := store.GetByID(ctx, resource.ID)
			require.NoError(t, err)
			require.NotNil(t, dbResource)
			require.Equal(t, resource, dbResource)
		})

		t.Run("GetByName", func(t *testing.T) {
			t.Parallel()

			dbResource, err := store.GetByName(ctx, resource.Name)
			require.NoError(t, err)
			require.NotNil(t, dbResource)
			require.Equal(t, resource, dbResource)
		})
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		t.Run("NotFound", func(t *testing.T) {
			t.Parallel()

			resource := &model.Resources{
				ID:          xuuid.New(),
				Name:        "Name" + t.Name(),
				Description: "Description" + t.Name(),
				ExternalID:  lo.ToPtr(xuuid.NewString()),
			}
			t.Run("GetByID", func(t *testing.T) {
				t.Parallel()

				dbResource, err := store.GetByID(ctx, resource.ID)
				require.Nil(t, dbResource)
				require.EqualError(t, err, apperr.ErrResourceNotFound.Error())
			})

			t.Run("GetByName", func(t *testing.T) {
				t.Parallel()

				dbResource, err := store.GetByName(ctx, resource.Name)
				require.Nil(t, dbResource)
				require.EqualError(t, err, apperr.ErrResourceNotFound.Error())
			})
		})
	})
}

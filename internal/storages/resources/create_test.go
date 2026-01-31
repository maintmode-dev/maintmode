package resources

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		resource := &entity.ResourceDetails{
			ID:          xuuid.New(),
			Name:        "Name" + t.Name(),
			Description: "Description" + t.Name(),
			ExternalID:  lo.ToPtr(xuuid.NewString()),
			CreatedAt:   xtime.UTCNow(),
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

package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := refreshtoken.NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		user := testdbutils.MakeUser(ctx, t, userStore)

		refreshToken := &entity.RefreshToken{
			Token:      uuid.NewString(),
			UserID:     user.ID,
			Family:     uuid.New(),
			ExpiresAt:  xtime.UTCNow().Add(time.Hour),
			GraceTTL:   nil,
			Revoked:    false,
			ReplacedBy: nil,
			BoundIP:    "BoundIP",
		}

		err := store.Save(ctx, refreshToken)
		require.NoError(t, err)

		dbToken, err := store.GetByTokenHash(ctx, refreshToken.Token)
		require.NoError(t, err)
		require.NotNil(t, dbToken)

		require.True(t, dbToken.CreatedAt.After(xtime.UTCNow().Add(-time.Minute)))
		refreshToken.CreatedAt = dbToken.CreatedAt
		require.Equal(t, refreshToken, dbToken)
	})
}

package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := refreshtoken.NewStore(db)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		token := makeRefreshToken(ctx, t, store)
		token.Revoked = true
		token.ReplacedBy = lo.ToPtr(uuid.NewString())
		token.GraceTTL = lo.ToPtr(xtime.UTCNow().Add(entity.DefaultGraceTTL).Round(time.Microsecond))

		err := store.Update(ctx, token)
		require.NoError(t, err)

		dbToken, err := store.GetByTokenHash(ctx, token.Token)
		require.NoError(t, err)
		require.NotNil(t, dbToken)
		require.Equal(t, token.Revoked, dbToken.Revoked)
		require.Equal(t, token.ReplacedBy, dbToken.ReplacedBy)
		require.Equal(t, token.GraceTTL, dbToken.GraceTTL)
		require.True(t, dbToken.UpdatedAt.After(xtime.UTCNow().Add(-time.Minute)))
	})
}

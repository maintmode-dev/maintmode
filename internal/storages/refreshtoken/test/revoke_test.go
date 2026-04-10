package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestRevoke(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := refreshtoken.NewStore(db)

	t.Run("RevokeByUserID", func(t *testing.T) {
		t.Parallel()

		token := makeRefreshToken(ctx, t, store)

		err := store.RevokeByUserID(ctx, token.UserID)
		require.NoError(t, err)

		dbToken, err := store.GetByTokenHash(ctx, token.Token)
		require.NoError(t, err)
		require.NotNil(t, dbToken)
		require.True(t, dbToken.Revoked)
		require.True(t, dbToken.UpdatedAt.After(xtime.UTCNow().Add(-time.Minute)))
	})

	t.Run("RevokeFamily", func(t *testing.T) {
		t.Parallel()

		token := makeRefreshToken(ctx, t, store)

		err := store.RevokeFamily(ctx, token.Family)
		require.NoError(t, err)

		dbToken, err := store.GetByTokenHash(ctx, token.Token)
		require.NoError(t, err)
		require.NotNil(t, dbToken)
		require.True(t, dbToken.Revoked)
		require.True(t, dbToken.UpdatedAt.After(xtime.UTCNow().Add(-time.Minute)))
	})
}

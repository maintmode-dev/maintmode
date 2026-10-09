package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := refreshtoken.NewStore(db)

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		dbToken, err := store.GetByTokenHash(ctx, "some token hash")
		require.Nil(t, dbToken)
		require.EqualError(t, err, apperr.ErrRefreshTokenNotFound.Error())
	})
}

func TestHasLiveToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := refreshtoken.NewStore(db)

	t.Run("live", func(t *testing.T) {
		t.Parallel()

		token := makeRefreshToken(ctx, t, store)

		live, err := store.HasLiveToken(ctx, token.Family)
		require.NoError(t, err)
		require.True(t, live)
	})

	t.Run("revoked", func(t *testing.T) {
		t.Parallel()

		token := makeRefreshToken(ctx, t, store)
		require.NoError(t, store.RevokeFamily(ctx, token.Family))

		live, err := store.HasLiveToken(ctx, token.Family)
		require.NoError(t, err)
		require.False(t, live)
	})

	t.Run("expired", func(t *testing.T) {
		t.Parallel()

		token := makeRefreshToken(ctx, t, store)
		_, err := db.ExecContext(ctx,
			`UPDATE refresh_tokens SET expires_at = $1 WHERE token_hash = $2`,
			xtime.UTCNow().Add(-time.Minute), token.Token)
		require.NoError(t, err)

		live, err := store.HasLiveToken(ctx, token.Family)
		require.NoError(t, err)
		require.False(t, live)
	})

	t.Run("unknown family", func(t *testing.T) {
		t.Parallel()

		live, err := store.HasLiveToken(ctx, uuid.New())
		require.NoError(t, err)
		require.False(t, live)
	})
}

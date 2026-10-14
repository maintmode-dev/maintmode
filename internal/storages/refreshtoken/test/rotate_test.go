package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/refreshtoken"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestRotate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := refreshtoken.NewStore(db)

	graceUntil := func() time.Time {
		return xtime.UTCNow().Add(entity.DefaultGraceTTL).Round(time.Microsecond)
	}

	t.Run("live token is rotated", func(t *testing.T) {
		t.Parallel()

		token := makeRefreshToken(ctx, t, store)
		successor, grace := uuid.NewString(), graceUntil()

		require.NoError(t, store.Rotate(ctx, token.Token, successor, grace))

		dbToken, err := store.GetByTokenHash(ctx, token.Token)
		require.NoError(t, err)
		require.True(t, dbToken.Revoked)
		require.Equal(t, successor, *dbToken.ReplacedBy)
		require.True(t, grace.Equal(*dbToken.GraceTTL))
	})

	t.Run("already rotated token is left alone", func(t *testing.T) {
		t.Parallel()

		token := makeRefreshToken(ctx, t, store)
		first, firstGrace := uuid.NewString(), graceUntil()
		require.NoError(t, store.Rotate(ctx, token.Token, first, firstGrace))

		err := store.Rotate(ctx, token.Token, uuid.NewString(), firstGrace.Add(time.Hour))
		require.ErrorIs(t, err, apperr.ErrRefreshTokenAlreadyRotated)

		dbToken, err := store.GetByTokenHash(ctx, token.Token)
		require.NoError(t, err)
		require.Equal(t, first, *dbToken.ReplacedBy, "the first successor must stand")
		require.True(t, firstGrace.Equal(*dbToken.GraceTTL), "the first grace window must stand")
	})

	t.Run("ended token is not brought back", func(t *testing.T) {
		t.Parallel()

		token := makeRefreshToken(ctx, t, store)
		require.NoError(t, store.RevokeFamily(ctx, token.Family))

		err := store.Rotate(ctx, token.Token, uuid.NewString(), graceUntil())
		require.ErrorIs(t, err, apperr.ErrRefreshTokenAlreadyRotated)

		dbToken, err := store.GetByTokenHash(ctx, token.Token)
		require.NoError(t, err)
		require.Nil(t, dbToken.ReplacedBy, "a logged-out token must not gain a successor")
	})
}

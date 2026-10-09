package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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
		// A rotated successor of the same session: the family is reported once.
		successor := *token
		successor.Token = uuid.NewString()
		require.NoError(t, store.Save(ctx, &successor))

		families, err := store.RevokeByUserID(ctx, token.UserID)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{token.Family}, families)

		dbToken, err := store.GetByTokenHash(ctx, token.Token)
		require.NoError(t, err)
		require.NotNil(t, dbToken)
		require.True(t, dbToken.Revoked)
		require.True(t, dbToken.UpdatedAt.After(xtime.UTCNow().Add(-time.Minute)))
	})

	t.Run("RevokeByUserIDExceptFamily", func(t *testing.T) {
		t.Parallel()

		kept := makeRefreshToken(ctx, t, store)
		other := *kept
		other.Token = uuid.NewString()
		other.Family = uuid.New()
		require.NoError(t, store.Save(ctx, &other))

		// The returned sessions are the ones revoked -- never the spared one,
		// whose access tokens must keep working.
		families, err := store.RevokeByUserIDExceptFamily(ctx, kept.UserID, kept.Family)
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{other.Family}, families)

		dbKept, err := store.GetByTokenHash(ctx, kept.Token)
		require.NoError(t, err)
		require.False(t, dbKept.Revoked)

		// Nothing matched: an empty result, not an error.
		families, err = store.RevokeByUserID(ctx, uuid.New())
		require.NoError(t, err)
		require.Empty(t, families)
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

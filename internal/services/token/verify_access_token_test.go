package token

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestVerifyAccessToken_WrongKey(t *testing.T) {
	ctx := context.Background()

	t.Run("wrong key", func(t *testing.T) {
		srv1 := initService(t)
		srv2 := initService(t)

		tokenStr, err := srv1.IssueAccessToken(ctx, tokenTTL, testUser(t))
		require.NoError(t, err)

		claims, err := srv2.VerifyAccessToken(ctx, tokenStr)
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessTokenToken)
	})

	t.Run("expired", func(t *testing.T) {
		srv := initService(t)
		srv.getNowF = func() time.Time { return xtime.UTCNow().Add(-1 * time.Hour) }

		token, err := srv.IssueAccessToken(ctx, tokenTTL, testUser(t))
		require.NoError(t, err)
		require.NotEmpty(t, token)

		claims, err := srv.VerifyAccessToken(ctx, token)
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrTokenExpired)
	})

	t.Run("wrong algorithm", func(t *testing.T) {
		srv := initService(t)

		// Sign with HMAC instead of ECDSA
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "u1"})
		tokenStr, err := token.SignedString([]byte("secret"))
		require.NoError(t, err)

		claims, err := srv.VerifyAccessToken(ctx, tokenStr)
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessTokenToken)
	})
}

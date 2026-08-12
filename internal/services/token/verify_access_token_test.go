package token

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
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
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
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
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("foreign issuer", func(t *testing.T) {
		srv := initService(t)

		// Correctly signed by this very key, so only the issuer check can reject
		// it: a token minted by another service that happens to share the key
		// must not authenticate here.
		token := jwt.NewWithClaims(jwt.SigningMethodES256, &entity.AccessClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   uuid.NewString(),
				Issuer:    "some-other-issuer",
				ExpiresAt: jwt.NewNumericDate(xtime.UTCNow().Add(tokenTTL)),
			},
		})
		tokenStr, err := token.SignedString(srv.privateKey)
		require.NoError(t, err)

		claims, err := srv.VerifyAccessToken(ctx, tokenStr)
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("missing expiration", func(t *testing.T) {
		srv := initService(t)

		// Without WithExpirationRequired an absent exp is not an error but an
		// absence of one — the token would verify and never expire.
		token := jwt.NewWithClaims(jwt.SigningMethodES256, &entity.AccessClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: uuid.NewString(),
				Issuer:  "test-issuer",
			},
		})
		tokenStr, err := token.SignedString(srv.privateKey)
		require.NoError(t, err)

		claims, err := srv.VerifyAccessToken(ctx, tokenStr)
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("a token this service issued still verifies", func(t *testing.T) {
		srv := initService(t)

		// The strict options must not invalidate our own tokens: issuer and exp
		// come from IssueAccessToken, and this is what proves the three options
		// agree with it.
		tokenStr, err := srv.IssueAccessToken(ctx, tokenTTL, testUser(t))
		require.NoError(t, err)

		claims, err := srv.VerifyAccessToken(ctx, tokenStr)
		require.NoError(t, err)
		require.NotNil(t, claims)
	})
}

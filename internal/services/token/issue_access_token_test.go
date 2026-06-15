package token

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestIssueAndVerifyAccessToken(t *testing.T) {
	ctx := context.Background()
	srv := initService(t)

	t.Run("ok", func(t *testing.T) {
		user := testUser(t)

		tokenStr, err := srv.IssueAccessToken(ctx, tokenTTL, user)
		require.NoError(t, err)

		claims, err := srv.VerifyAccessToken(ctx, tokenStr)
		require.NoError(t, err)
		require.Equal(t, user.ID.String(), claims.Subject)
		require.Equal(t, user.Email, claims.UserEmail)
		require.Equal(t, srv.issuer, claims.Issuer)
		require.Equal(t, user.Roles, claims.UserRoles)
		require.NotEmpty(t, claims.ID)
		require.NotNil(t, claims.ExpiresAt)
		ttl := claims.ExpiresAt.Sub(xtime.UTCNow())
		require.GreaterOrEqual(t, ttl, tokenTTL-time.Minute)
		require.LessOrEqual(t, ttl, tokenTTL+time.Minute)

		require.NotNil(t, claims.IssuedAt)
		require.True(t, claims.IssuedAt.After(xtime.UTCNow().Add(-time.Minute)))
	})

	t.Run("name from user", func(t *testing.T) {
		user := testUser(t)
		user.Name = "Alice Liddell"

		tokenStr, err := srv.IssueAccessToken(ctx, tokenTTL, user)
		require.NoError(t, err)

		claims, err := srv.VerifyAccessToken(ctx, tokenStr)
		require.NoError(t, err)
		require.Equal(t, "Alice Liddell", claims.UserName)
	})

	t.Run("empty name falls back to email", func(t *testing.T) {
		user := testUser(t)
		user.Name = ""

		tokenStr, err := srv.IssueAccessToken(ctx, tokenTTL, user)
		require.NoError(t, err)

		claims, err := srv.VerifyAccessToken(ctx, tokenStr)
		require.NoError(t, err)
		require.Equal(t, user.Email, claims.UserName)
	})

	t.Run("blocked user is rejected", func(t *testing.T) {
		user := testUser(t)
		blockedAt := xtime.UTCNow()
		user.BlockedAt = &blockedAt

		_, err := srv.IssueAccessToken(ctx, tokenTTL, user)
		require.ErrorIs(t, err, apperr.ErrUserBlocked)
	})

	t.Run("has kid", func(t *testing.T) {
		tokenStr, err := srv.IssueAccessToken(ctx, tokenTTL, testUser(t))
		require.NoError(t, err)

		parsed, _, err := jwt.NewParser().ParseUnverified(tokenStr, &entity.AccessClaims{})
		require.NoError(t, err)

		value, ok := parsed.Header["kid"]
		require.True(t, ok)
		require.NotNil(t, value)

		kid, ok := value.(string)
		require.True(t, ok)
		require.NotEmpty(t, kid)
		require.Equal(t, srv.kid, kid)
	})
}

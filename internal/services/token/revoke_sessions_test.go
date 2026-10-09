package token

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	"github.com/ruko1202/maintmode/internal/storages/users"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
	testdbutils "github.com/ruko1202/maintmode/test/utils/db"
)

// TestRevokeMarksSessions pins which sessions each revocation path marks: the
// ones it ended, and never the one it spared.
func TestRevokeMarksSessions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	blacklist := blacklisttoken.NewStore(valkey)

	sessionRevoked := func(t *testing.T, family uuid.UUID) bool {
		t.Helper()
		revoked, err := blacklist.IsRevoked(ctx, xuuid.NewString(), family.String())
		require.NoError(t, err)
		return revoked
	}

	seedSession := func(t *testing.T, srv *Service, userID uuid.UUID) *entity.RefreshToken {
		t.Helper()
		hash := seedRefreshToken(ctx, t, srv, userID, xtime.UTCNow().Add(time.Hour), false)
		rt, err := srv.tokensStore.GetByTokenHash(ctx, hash)
		require.NoError(t, err)
		return rt
	}

	t.Run("by user except the caller's session", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		user := testdbutils.MakeUser(ctx, t, users.NewStore(db))
		kept := seedSession(t, srv, user.ID)
		other := seedSession(t, srv, user.ID)

		require.NoError(t, srv.RevokeRefreshTokenByUserIDExceptFamily(ctx, user.ID, kept.Family))

		require.True(t, sessionRevoked(t, other.Family))
		require.False(t, sessionRevoked(t, kept.Family), "the session that changed the password keeps working")
	})

	t.Run("by user", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		user := testdbutils.MakeUser(ctx, t, users.NewStore(db))
		first := seedSession(t, srv, user.ID)
		second := seedSession(t, srv, user.ID)

		require.NoError(t, srv.RevokeRefreshTokenByUserID(ctx, user.ID))

		require.True(t, sessionRevoked(t, first.Family))
		require.True(t, sessionRevoked(t, second.Family))
	})

	t.Run("by refresh token, even a rotated one", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		user := testdbutils.MakeUser(ctx, t, users.NewStore(db))
		owner := &entity.AccessClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: user.ID.String()}}

		// A token already rotated away still names its session, and logout
		// with it ends the whole session -- access tokens included.
		raw, hashed, err := srv.GenerateRefreshToken(ctx)
		require.NoError(t, err)
		family := uuid.New()
		require.NoError(t, srv.SaveRefreshToken(ctx, &entity.RefreshToken{
			Token:            hashed,
			UserID:           user.ID,
			Family:           family,
			ExpiresAt:        xtime.UTCNow().Add(time.Hour),
			Revoked:          true,
			SessionStartedAt: xtime.UTCNow(),
		}))

		revoked, err := srv.RevokeFamilyByRefreshToken(ctx, raw, owner)
		require.NoError(t, err)
		require.Equal(t, family, revoked)
		require.True(t, sessionRevoked(t, family))
	})

	t.Run("not when the revocation is refused", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		user := testdbutils.MakeUser(ctx, t, users.NewStore(db))
		session := seedSession(t, srv, user.ID)
		stranger := &entity.AccessClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: uuid.NewString()}}

		raw, hashed, err := srv.GenerateRefreshToken(ctx)
		require.NoError(t, err)
		require.NoError(t, srv.SaveRefreshToken(ctx, &entity.RefreshToken{
			Token:            hashed,
			UserID:           user.ID,
			Family:           session.Family,
			ExpiresAt:        xtime.UTCNow().Add(time.Hour),
			SessionStartedAt: xtime.UTCNow(),
		}))

		_, err = srv.RevokeFamilyByRefreshToken(ctx, raw, stranger)
		require.Error(t, err)
		require.False(t, sessionRevoked(t, session.Family))
	})

	t.Run("by session", func(t *testing.T) {
		t.Parallel()

		srv := initService(t)
		user := testdbutils.MakeUser(ctx, t, users.NewStore(db))
		ended := seedSession(t, srv, user.ID)
		other := seedSession(t, srv, user.ID)

		require.NoError(t, srv.RevokeRefreshTokenByFamily(ctx, ended.Family))

		require.True(t, sessionRevoked(t, ended.Family))
		require.False(t, sessionRevoked(t, other.Family), "the user's other sessions go on")
	})
}

// TestTokenWithoutSessionIsCheckedByJTI pins the rollout decision: an access
// token minted before the sid claim existed still verifies, and is revocable
// only by its jti until it expires.
func TestTokenWithoutSessionIsCheckedByJTI(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := initService(t)
	blacklist := blacklisttoken.NewStore(valkey)

	now := xtime.UTCNow()
	legacy := jwt.NewWithClaims(jwt.SigningMethodES256, entity.AccessClaims{
		UserName:  "alice",
		UserEmail: "alice@example.com",
		UserRoles: entity.DefaultRoles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        xuuid.NewString(),
			Subject:   uuid.NewString(),
			Issuer:    srv.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	})
	signed, err := legacy.SignedString(srv.privateKey)
	require.NoError(t, err)

	claims, err := srv.VerifyAccessToken(ctx, signed)
	require.NoError(t, err)
	require.Empty(t, claims.SessionID)

	revoked, err := blacklist.IsRevoked(ctx, claims.ID, claims.SessionID)
	require.NoError(t, err)
	require.False(t, revoked)

	require.NoError(t, blacklist.Add(ctx, claims.ID, time.Minute))
	revoked, err = blacklist.IsRevoked(ctx, claims.ID, claims.SessionID)
	require.NoError(t, err)
	require.True(t, revoked)
}

func TestIssueAccessTokenRequiresSession(t *testing.T) {
	t.Parallel()

	_, err := initService(t).IssueAccessToken(context.Background(), tokenTTL, testUser(t), uuid.Nil)
	require.ErrorIs(t, err, errNoSession)
}

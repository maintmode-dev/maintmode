package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestLogout(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		err = srv.Logout(ctx, pair)
		require.NoError(t, err)

		// Refresh token should be revoked
		rt, err := srv.tokenSrv.GetRefreshToken(ctx, pair.RefreshToken)
		require.NoError(t, err)
		require.True(t, rt.Revoked)

		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		ok, err := srv.blacklistStore.Contains(ctx, claims.ID)
		require.NoError(t, err)
		require.True(t, ok)
	})

	// Logout ends the session, not just the token it was handed: a tab still
	// holding the predecessor of a rotation logs out the rotated successor too.
	t.Run("revokes the whole family", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		srv.cfg.RefreshTokenGracePeriod = 30 * time.Second

		exchangeIDTokenMock(mocks, 2)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		other, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		rotated, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)

		require.NoError(t, srv.Logout(ctx, pair), "logging out with the rotated-out pair")

		predecessor, err := srv.tokenSrv.GetRefreshToken(ctx, pair.RefreshToken)
		require.NoError(t, err)
		require.True(t, predecessor.Revoked)
		require.NotNil(t, predecessor.ReplacedBy, "logout must not erase the rotation record")

		current, err := srv.tokenSrv.GetRefreshToken(ctx, rotated.RefreshToken)
		require.NoError(t, err)
		require.True(t, current.Revoked, "the successor belongs to the same session")
		require.Nil(t, current.ReplacedBy)

		_, err = srv.Refresh(ctx, rotated.RefreshToken, "10.0.0.1", "")
		require.ErrorIs(t, err, apperr.ErrLogoutAlready)

		untouched, err := srv.tokenSrv.GetRefreshToken(ctx, other.RefreshToken)
		require.NoError(t, err)
		require.False(t, untouched.Revoked, "another session of the same user must survive")
	})

	t.Run("empty token", func(t *testing.T) {
		t.Parallel()

		t.Run("RefreshToken", func(t *testing.T) {
			t.Parallel()

			srv, mocks := initService(t)

			exchangeIDTokenMock(mocks, 1)

			pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
				Provider: entity.AuthMethodGoogle,
				IDToken:  "id-token",
				ClientIP: "10.0.0.1",
			})
			require.NoError(t, err)

			err = srv.Logout(ctx, &entity.TokenPair{
				AccessToken:  pair.AccessToken,
				RefreshToken: "",
			})
			require.NoError(t, err)

			claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
			require.NoError(t, err)
			ok, err := srv.blacklistStore.Contains(ctx, claims.ID)
			require.NoError(t, err)
			require.True(t, ok)
		})

		t.Run("AccessToken", func(t *testing.T) {
			t.Parallel()

			srv, mocks := initService(t)

			exchangeIDTokenMock(mocks, 1)

			pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
				Provider: entity.AuthMethodGoogle,
				IDToken:  "id-token",
				ClientIP: "10.0.0.1",
			})
			require.NoError(t, err)

			err = srv.Logout(ctx, &entity.TokenPair{
				AccessToken:  "",
				RefreshToken: pair.RefreshToken,
			})
			require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
		})
	})

	t.Run("ownership mismatch", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		exchangeIDTokenMock(mocks, 1)

		pair1, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		exchangeIDTokenMock(mocks, 1)

		pair2, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		err = srv.Logout(ctx, &entity.TokenPair{
			AccessToken:  pair1.AccessToken,
			RefreshToken: pair2.RefreshToken,
		})
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)

		for _, pair := range []*entity.TokenPair{pair1, pair2} {
			rt, err := srv.tokenSrv.GetRefreshToken(ctx, pair.RefreshToken)
			require.NoError(t, err)
			require.False(t, rt.Revoked)

			claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
			require.NoError(t, err)
			ok, err := srv.blacklistStore.Contains(ctx, claims.ID)
			require.NoError(t, err)
			require.False(t, ok)
		}
	})
}

func TestLogoutAll(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		exchangeIDTokenMock(mocks, 2)

		pair1, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		pair2, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		err = srv.LogoutAll(ctx, pair1.AccessToken)
		require.NoError(t, err)

		// Both refresh tokens should be revoked
		for _, raw := range []string{pair1.RefreshToken, pair2.RefreshToken} {
			rt, err := srv.tokenSrv.GetRefreshToken(ctx, raw)
			require.NoError(t, err)
			require.True(t, rt.Revoked)
		}

		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair1.AccessToken)
		require.NoError(t, err)
		ok, err := srv.blacklistStore.Contains(ctx, claims.ID)
		require.NoError(t, err)
		require.True(t, ok)
	})
}

// TestLogoutAuditSessionID pins what a logout's audit row names as its session:
// the refresh-token family, the id login.success recorded for the same session,
// so an admin can pair the two rows. Only a token minted before access tokens
// carried their session falls back to its jti.
func TestLogoutAuditSessionID(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	logoutSessionID := func(t *testing.T, publisher *recordingAuditPublisher) string {
		t.Helper()

		var found []audit.LogoutSuccess
		for _, action := range publisher.actions() {
			if logout, ok := action.(audit.LogoutSuccess); ok {
				found = append(found, logout)
			}
		}
		require.Len(t, found, 1)

		return found[0].SessionID
	}

	t.Run("logout records the session family", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 1)
		pair := openSession(ctx, t, srv)
		// Rotated, so the access token's jti and the family differ from
		// anything minted at sign-in.
		rotated, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)

		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher
		require.NoError(t, srv.Logout(ctx, rotated))

		require.Equal(t, pair.SessionID.String(), logoutSessionID(t, publisher))
	})

	t.Run("logout-all records the session it was made from", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 2)
		laptop := openSession(ctx, t, srv)
		phone := openSession(ctx, t, srv)
		requireSameUser(ctx, t, srv, laptop, phone)

		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher
		require.NoError(t, srv.LogoutAll(ctx, phone.AccessToken))

		require.Equal(t, phone.SessionID.String(), logoutSessionID(t, publisher))
	})

	t.Run("a token without a session records its jti", func(t *testing.T) {
		t.Parallel()

		key := newSigningKey(t)
		srv, mocks := initServiceWithDeps(t, entity.AuthMethodGoogle, serviceDeps{signingKey: key})
		exchangeIDTokenMock(mocks, 1)
		pair := openSession(ctx, t, srv)
		legacy, jti := legacyAccessToken(ctx, t, srv, key, pair.AccessToken)

		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher
		require.NoError(t, srv.Logout(ctx, &entity.TokenPair{AccessToken: legacy}))

		require.Equal(t, jti, logoutSessionID(t, publisher))
	})
}

func newSigningKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return key
}

// legacyAccessToken re-signs a live access token of srv the way releases before
// the sid claim minted it: same user, a fresh jti, no session. It returns the
// token and its jti.
func legacyAccessToken(
	ctx context.Context, t *testing.T, srv *Service, key *ecdsa.PrivateKey, accessToken string,
) (signed, jti string) {
	t.Helper()

	claims, err := srv.tokenSrv.VerifyAccessToken(ctx, accessToken)
	require.NoError(t, err)

	claims.SessionID = ""
	claims.ID = xuuid.NewString()

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = testKID
	signed, err = token.SignedString(key)
	require.NoError(t, err)

	verified, err := srv.tokenSrv.VerifyAccessToken(ctx, signed)
	require.NoError(t, err)
	require.Empty(t, verified.SessionID)

	return signed, claims.ID
}

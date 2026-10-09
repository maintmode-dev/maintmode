package auth

import (
	"context"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
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

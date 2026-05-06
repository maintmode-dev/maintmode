package auth

import (
	"context"
	"testing"

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

		handleCallbackMock(mocks, 1)

		pair, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
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

	t.Run("empty token", func(t *testing.T) {
		t.Parallel()

		t.Run("RefreshToken", func(t *testing.T) {
			t.Parallel()

			srv, mocks := initService(t)

			handleCallbackMock(mocks, 1)

			pair, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
				Provider:     entity.OAuthProviderGoogle,
				CallbackCode: "code-1",
				ClientIP:     "10.0.0.1",
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

			handleCallbackMock(mocks, 1)

			pair, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
				Provider:     entity.OAuthProviderGoogle,
				CallbackCode: "code-1",
				ClientIP:     "10.0.0.1",
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

		handleCallbackMock(mocks, 1)

		pair1, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
		})
		require.NoError(t, err)

		handleCallbackMock(mocks, 1)

		pair2, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
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

		handleCallbackMock(mocks, 2)

		pair1, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
		})
		require.NoError(t, err)

		pair2, err := srv.HandleOAuthCallback(ctx, &entity.HandleOAuthCallbackCmd{
			Provider:     entity.OAuthProviderGoogle,
			CallbackCode: "code-1",
			ClientIP:     "10.0.0.1",
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

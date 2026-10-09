package auth

import (
	"context"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func TestRefresh(t *testing.T) {
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

		newPair, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
		require.NoError(t, err)
		require.NotNil(t, newPair)
		require.NotEmpty(t, newPair.RefreshToken)
		require.NotEmpty(t, newPair.AccessToken)
		require.NotEmpty(t, newPair.ExpiresIn)
		require.NotEqualValues(t, pair.RefreshToken, newPair.RefreshToken)
		require.NotEqualValues(t, pair.AccessToken, newPair.AccessToken)
	})

	t.Run("logout already", func(t *testing.T) {
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

		newPair, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
		require.ErrorIs(t, err, apperr.ErrLogoutAlready)
		require.Nil(t, newPair)
	})

	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)

		newPair, err := srv.Refresh(ctx, "nonexistent", "10.0.0.1")
		require.ErrorIs(t, err, apperr.ErrInvalidRefreshToken)
		require.Nil(t, newPair)
	})

	// A session is not bound to an address: a refresh from a new one rotates
	// as usual, inside the grace window and out of it.
	t.Run("new address keeps the session", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		// A real grace window: the second refresh below must land inside it.
		srv.cfg.RefreshTokenGracePeriod = 30 * time.Second

		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		rotated, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.10")
		require.NoError(t, err)
		require.NotEmpty(t, rotated.RefreshToken)

		// Within the grace period, from yet another address.
		graced, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.20")
		require.NoError(t, err)
		require.NotEmpty(t, graced.AccessToken)

		again, err := srv.Refresh(ctx, rotated.RefreshToken, "10.0.0.30")
		require.NoError(t, err)
		require.NotEmpty(t, again.RefreshToken)

		live, err := srv.tokenSrv.GetRefreshToken(ctx, again.RefreshToken)
		require.NoError(t, err)
		require.False(t, live.Revoked, "the session must survive every change of address")
	})

	t.Run("grace period", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			t.Parallel()

			srv, mocks := initService(t)
			srv.cfg.RefreshTokenGracePeriod = 30 * time.Second

			exchangeIDTokenMock(mocks, 1)

			pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
				Provider: entity.AuthMethodGoogle,
				IDToken:  "id-token",
				ClientIP: "10.0.0.1",
			})
			require.NoError(t, err)

			// First refresh succeeds and rotates
			newPair1, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
			require.NoError(t, err)
			require.NotNil(t, newPair1)

			rt, err := srv.tokenSrv.GetRefreshToken(ctx, pair.RefreshToken)
			require.NoError(t, err)
			require.True(t, rt.Revoked)

			//Reuse old token → should detect reuse
			newPair2, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
			require.NoError(t, err)
			require.NotNil(t, newPair2)
			require.Empty(t, newPair2.RefreshToken)
			require.NotEmpty(t, newPair2.AccessToken)
			require.NotEmpty(t, newPair2.ExpiresIn)
			require.NotEqualValues(t, newPair1.RefreshToken, newPair2.RefreshToken)
			require.NotEqualValues(t, newPair1.AccessToken, newPair2.AccessToken)
		})

		// A token rotated a moment before a logout must not keep minting access
		// tokens through its grace window: whoever stole it would outlive the
		// logout that was meant to end the session.
		t.Run("closed by a later logout", func(t *testing.T) {
			t.Parallel()

			srv, mocks := initService(t)
			srv.cfg.RefreshTokenGracePeriod = 30 * time.Second

			exchangeIDTokenMock(mocks, 1)

			pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
				Provider: entity.AuthMethodGoogle,
				IDToken:  "id-token",
				ClientIP: "10.0.0.1",
			})
			require.NoError(t, err)

			rotated, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
			require.NoError(t, err)

			require.NoError(t, srv.Logout(ctx, rotated))

			stale, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
			require.Error(t, err, "the predecessor's grace window must close with the session")
			require.Nil(t, stale)
		})

		t.Run("reuse detection", func(t *testing.T) {
			t.Parallel()

			srv, mocks := initService(t)

			exchangeIDTokenMock(mocks, 1)

			pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
				Provider: entity.AuthMethodGoogle,
				IDToken:  "id-token",
				ClientIP: "10.0.0.1",
			})
			require.NoError(t, err)

			// First refresh succeeds and rotates
			newPair, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
			require.NoError(t, err)
			require.NotNil(t, newPair)

			rt, err := srv.tokenSrv.GetRefreshToken(ctx, pair.RefreshToken)
			require.NoError(t, err)
			require.True(t, rt.Revoked)
			// Wait past grace period simulation: manually expire the grace TTL
			rt.GraceTTL = lo.ToPtr(rt.GraceTTL.Add(-srv.cfg.RefreshTokenGracePeriod))
			err = srv.tokenSrv.UpdateRefreshToken(ctx, rt)
			require.NoError(t, err)

			//Reuse old token → should detect reuse
			newPair, err = srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
			require.ErrorIs(t, err, apperr.ErrTokenReuse)
			require.Nil(t, newPair)
		})
	})

	t.Run("revoked and expired", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		// Rotate the token
		newPair1, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
		require.NoError(t, err)
		require.NotNil(t, newPair1)

		rt, err := srv.tokenSrv.GetRefreshToken(ctx, pair.RefreshToken)
		require.NoError(t, err)
		require.True(t, rt.Revoked)
		// Mark old token as expired AND past grace period
		rt.GraceTTL = lo.ToPtr(xtime.UTCNow())
		rt.ExpiresAt = xtime.UTCNow()
		err = srv.tokenSrv.UpdateRefreshToken(ctx, rt)
		require.NoError(t, err)

		//Reuse old token → should detect reuse
		newPair2, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
		require.ErrorIs(t, err, apperr.ErrTokenReuse)
		require.Nil(t, newPair2)
	})

	t.Run("concurrent lock busy", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		eg := new(errgroup.Group)
		for range 5 {
			eg.Go(func() error {
				_, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1")
				return err
			})
		}

		require.ErrorIs(t, eg.Wait(), apperr.ErrLockBusy)
	})
}

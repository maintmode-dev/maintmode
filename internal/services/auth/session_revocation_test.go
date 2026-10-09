package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// TestSessionRevocationCutsOffAccessTokens pins that ending a session ends its
// access tokens on the spot rather than when they expire: every token minted
// for the session -- at sign-in, on rotation, inside a grace window -- is
// refused by the write gate as soon as the session is revoked, by any path.
func TestSessionRevocationCutsOffAccessTokens(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("reuse revocation", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 1)

		pair := openSession(ctx, t, srv)
		rotated, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)

		requireWriteStatus(t, srv, pair.AccessToken, http.StatusNoContent)
		requireWriteStatus(t, srv, rotated.AccessToken, http.StatusNoContent)

		// Replay the rotated-away token past its grace window.
		expireGrace(ctx, t, srv, pair.RefreshToken)
		_, err = srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		require.ErrorIs(t, err, apperr.ErrTokenReuse)

		// Both holders lose access now: the thief and the victim alike, since
		// the server cannot tell which is which.
		requireWriteStatus(t, srv, pair.AccessToken, http.StatusUnauthorized)
		requireWriteStatus(t, srv, rotated.AccessToken, http.StatusUnauthorized)
	})

	t.Run("logout takes the session's other access tokens", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		srv.cfg.RefreshTokenGracePeriod = 30 * time.Second
		exchangeIDTokenMock(mocks, 1)

		pair := openSession(ctx, t, srv)
		rotated, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)
		// A second refresh of the same token inside the grace window mints
		// another access token for the session.
		graced, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)
		require.NotEmpty(t, graced.AccessToken)

		requireWriteStatus(t, srv, pair.AccessToken, http.StatusNoContent)
		requireWriteStatus(t, srv, graced.AccessToken, http.StatusNoContent)

		require.NoError(t, srv.Logout(ctx, rotated))

		// Logout blacklists only the token it was called with; the rest go
		// with the session.
		requireWriteStatus(t, srv, rotated.AccessToken, http.StatusUnauthorized)
		requireWriteStatus(t, srv, pair.AccessToken, http.StatusUnauthorized)
		requireWriteStatus(t, srv, graced.AccessToken, http.StatusUnauthorized)
	})

	t.Run("logout leaves the user's other sessions alone", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 2)

		laptop := openSession(ctx, t, srv)
		phone := openSession(ctx, t, srv)
		requireSameUser(ctx, t, srv, laptop, phone)

		require.NoError(t, srv.Logout(ctx, laptop))

		requireWriteStatus(t, srv, laptop.AccessToken, http.StatusUnauthorized)
		requireWriteStatus(t, srv, phone.AccessToken, http.StatusNoContent)
	})

	t.Run("logout-all takes every session", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 2)

		laptop := openSession(ctx, t, srv)
		phone := openSession(ctx, t, srv)
		requireSameUser(ctx, t, srv, laptop, phone)

		require.NoError(t, srv.LogoutAll(ctx, laptop.AccessToken))

		requireWriteStatus(t, srv, laptop.AccessToken, http.StatusUnauthorized)
		requireWriteStatus(t, srv, phone.AccessToken, http.StatusUnauthorized)
	})

	t.Run("a token carries its session", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 1)

		pair := openSession(ctx, t, srv)
		rotated, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)

		for _, token := range []string{pair.AccessToken, rotated.AccessToken} {
			claims, err := srv.tokenSrv.VerifyAccessToken(ctx, token)
			require.NoError(t, err)
			require.Equal(t, pair.SessionID.String(), claims.SessionID)
		}
	})
}

func openSession(ctx context.Context, t *testing.T, srv *Service) *entity.TokenPair {
	t.Helper()

	pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
		Provider: entity.AuthMethodGoogle,
		IDToken:  "id-token",
		ClientIP: "10.0.0.1",
	})
	require.NoError(t, err)

	return pair
}

// expireGrace moves a rotated refresh token's grace window into the past, so
// the next refresh of it is reuse rather than a racing client.
func expireGrace(ctx context.Context, t *testing.T, srv *Service, refreshToken string) {
	t.Helper()

	rt, err := srv.tokenSrv.GetRefreshToken(ctx, refreshToken)
	require.NoError(t, err)
	require.True(t, rt.Revoked)
	rt.GraceTTL = lo.ToPtr(rt.GraceTTL.Add(-time.Hour))
	require.NoError(t, srv.tokenSrv.UpdateRefreshToken(ctx, rt))
}

func requireSameUser(ctx context.Context, t *testing.T, srv *Service, a, b *entity.TokenPair) {
	t.Helper()

	claimsA, err := srv.tokenSrv.VerifyAccessToken(ctx, a.AccessToken)
	require.NoError(t, err)
	claimsB, err := srv.tokenSrv.VerifyAccessToken(ctx, b.AccessToken)
	require.NoError(t, err)
	require.Equal(t, claimsA.Subject, claimsB.Subject)
	require.NotEqual(t, claimsA.SessionID, claimsB.SessionID)
}

// requireWriteStatus sends a write through the production access-token gates
// (RequireAccessToken, then RequireActiveToken backed by this service) and
// checks the status a client would see.
func requireWriteStatus(t *testing.T, srv *Service, accessToken string, want int) {
	t.Helper()

	e := echo.New()
	e.POST("/protected",
		func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) },
		middlewares.RequireAccessToken(srv.tokenSrv),
		middlewares.RequireActiveToken(srv),
	)

	req := httptest.NewRequest(http.MethodPost, "/protected", http.NoBody)
	xecho.SetBearerToken(req, accessToken)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, want, rec.Code, rec.Body.String())
}

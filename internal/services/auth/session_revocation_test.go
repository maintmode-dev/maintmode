package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	valkeyDB "github.com/redis/go-redis/v9"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
	"github.com/ruko1202/maintmode/internal/storages/blacklisttoken"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// TestSessionRevocationCutsOffAccessTokens pins that ending a session ends its
// access tokens on the spot rather than when they expire: every token minted
// for the session -- at sign-in, on rotation, inside a grace window -- is
// refused by the access-token gate, on reads and writes alike, as soon as the
// session is revoked, by any path.
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

		requireGateStatus(t, srv, pair.AccessToken, http.StatusNoContent)
		requireGateStatus(t, srv, rotated.AccessToken, http.StatusNoContent)

		// Replay the rotated-away token past its grace window.
		expireGrace(ctx, t, srv, pair.RefreshToken)
		_, err = srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		require.ErrorIs(t, err, apperr.ErrTokenReuse)

		// Both holders lose access now: the thief and the victim alike, since
		// the server cannot tell which is which.
		requireGateStatus(t, srv, pair.AccessToken, http.StatusUnauthorized)
		requireGateStatus(t, srv, rotated.AccessToken, http.StatusUnauthorized)
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

		requireGateStatus(t, srv, pair.AccessToken, http.StatusNoContent)
		requireGateStatus(t, srv, graced.AccessToken, http.StatusNoContent)

		require.NoError(t, srv.Logout(ctx, rotated))

		// Logout blacklists only the token it was called with; the rest go
		// with the session.
		requireGateStatus(t, srv, rotated.AccessToken, http.StatusUnauthorized)
		requireGateStatus(t, srv, pair.AccessToken, http.StatusUnauthorized)
		requireGateStatus(t, srv, graced.AccessToken, http.StatusUnauthorized)
	})

	t.Run("logout leaves the user's other sessions alone", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 2)

		laptop := openSession(ctx, t, srv)
		phone := openSession(ctx, t, srv)
		requireSameUser(ctx, t, srv, laptop, phone)

		require.NoError(t, srv.Logout(ctx, laptop))

		requireGateStatus(t, srv, laptop.AccessToken, http.StatusUnauthorized)
		requireGateStatus(t, srv, phone.AccessToken, http.StatusNoContent)
	})

	t.Run("logout without a refresh token ends the access token's session", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		srv.cfg.RefreshTokenGracePeriod = 30 * time.Second
		exchangeIDTokenMock(mocks, 2)

		laptop := openSession(ctx, t, srv)
		rotated, err := srv.Refresh(ctx, laptop.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)
		graced, err := srv.Refresh(ctx, laptop.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)
		phone := openSession(ctx, t, srv)
		requireSameUser(ctx, t, srv, laptop, phone)

		// The access token alone: the client has no refresh token to send.
		require.NoError(t, srv.Logout(ctx, &entity.TokenPair{AccessToken: rotated.AccessToken}))

		// Every token of the session is over: the access tokens on the spot,
		// the refresh tokens in Postgres. A grace re-issue mints an access
		// token only.
		for _, accessToken := range []string{laptop.AccessToken, rotated.AccessToken, graced.AccessToken} {
			requireGateStatus(t, srv, accessToken, http.StatusUnauthorized)
		}
		for _, refreshToken := range []string{laptop.RefreshToken, rotated.RefreshToken} {
			requireRefreshRevoked(ctx, t, srv, refreshToken, true)
		}
		_, err = srv.Refresh(ctx, rotated.RefreshToken, "10.0.0.1", "")
		require.ErrorIs(t, err, apperr.ErrLogoutAlready)

		// The user's other session is untouched.
		requireGateStatus(t, srv, phone.AccessToken, http.StatusNoContent)
		requireRefreshRevoked(ctx, t, srv, phone.RefreshToken, false)
	})

	t.Run("logout with a token minted before sid ends only that token", func(t *testing.T) {
		t.Parallel()

		key := newSigningKey(t)
		srv, mocks := initServiceWithDeps(t, entity.AuthMethodGoogle, serviceDeps{signingKey: key})
		exchangeIDTokenMock(mocks, 1)

		pair := openSession(ctx, t, srv)
		legacy, _ := legacyAccessToken(ctx, t, srv, key, pair.AccessToken)

		require.NoError(t, srv.Logout(ctx, &entity.TokenPair{AccessToken: legacy}))

		// No session to name, so nothing but the token itself goes: as before
		// access tokens carried their session.
		requireGateStatus(t, srv, legacy, http.StatusUnauthorized)
		requireGateStatus(t, srv, pair.AccessToken, http.StatusNoContent)
		requireRefreshRevoked(ctx, t, srv, pair.RefreshToken, false)
	})

	t.Run("logout naming two sessions ends both", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 3)

		laptop := openSession(ctx, t, srv)
		phone := openSession(ctx, t, srv)
		tablet := openSession(ctx, t, srv)
		requireSameUser(ctx, t, srv, laptop, phone)
		requireSameUser(ctx, t, srv, laptop, tablet)

		// A stale access token beside a refresh token of another session of
		// the same user: both are the caller's, and both end.
		require.NoError(t, srv.Logout(ctx, &entity.TokenPair{
			AccessToken:  laptop.AccessToken,
			RefreshToken: phone.RefreshToken,
		}))

		for _, pair := range []*entity.TokenPair{laptop, phone} {
			requireGateStatus(t, srv, pair.AccessToken, http.StatusUnauthorized)
			requireRefreshRevoked(ctx, t, srv, pair.RefreshToken, true)
		}

		requireGateStatus(t, srv, tablet.AccessToken, http.StatusNoContent)
		requireRefreshRevoked(ctx, t, srv, tablet.RefreshToken, false)
	})

	t.Run("logout-all takes every session", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 2)

		laptop := openSession(ctx, t, srv)
		phone := openSession(ctx, t, srv)
		requireSameUser(ctx, t, srv, laptop, phone)

		require.NoError(t, srv.LogoutAll(ctx, laptop.AccessToken))

		requireGateStatus(t, srv, laptop.AccessToken, http.StatusUnauthorized)
		requireGateStatus(t, srv, phone.AccessToken, http.StatusUnauthorized)
	})

	t.Run("a blacklisted jti is refused on its own", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 1)

		pair := openSession(ctx, t, srv)
		rotated, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)

		// Only the jti, as a logout of a token minted before the sid claim
		// leaves it: the session itself stays live.
		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, rotated.AccessToken)
		require.NoError(t, err)
		require.NoError(t, srv.blacklistStore.Add(ctx, claims.ID, time.Minute))

		requireGateStatus(t, srv, rotated.AccessToken, http.StatusUnauthorized)
		requireGateStatus(t, srv, pair.AccessToken, http.StatusNoContent)
	})

	t.Run("an unreachable blacklist fails closed", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 1)

		pair := openSession(ctx, t, srv)
		requireGateStatus(t, srv, pair.AccessToken, http.StatusNoContent)

		// Nothing listens on port 1: every lookup fails.
		down := valkeyDB.NewClient(&valkeyDB.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
		t.Cleanup(func() { _ = down.Close() })
		srv.blacklistStore = blacklisttoken.NewStore(down)

		requireGateStatus(t, srv, pair.AccessToken, http.StatusInternalServerError)
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

func requireRefreshRevoked(ctx context.Context, t *testing.T, srv *Service, refreshToken string, want bool) {
	t.Helper()

	rt, err := srv.tokenSrv.GetRefreshToken(ctx, refreshToken)
	require.NoError(t, err)
	require.Equal(t, want, rt.Revoked)
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

// requireGateStatus sends a write and a read through the production
// access-token gates (RequireAccessToken, then RequireActiveToken backed by this
// service) and checks that both get the status a client would see: a revoked
// token must not keep reading any more than writing.
func requireGateStatus(t *testing.T, srv *Service, accessToken string, want int) {
	t.Helper()

	for _, method := range []string{http.MethodPost, http.MethodGet} {
		e := echo.New()
		e.Add(method, "/protected",
			func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) },
			middlewares.RequireAccessToken(srv.tokenSrv),
			middlewares.RequireActiveToken(srv),
		)

		req := httptest.NewRequest(method, "/protected", http.NoBody)
		xecho.SetBearerToken(req, accessToken)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.Equal(t, want, rec.Code, "%s: %s", method, rec.Body.String())
	}
}

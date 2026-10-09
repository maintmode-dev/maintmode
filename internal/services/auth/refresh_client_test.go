package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/ruko1202/maintmode/internal/entity"
)

// observedCtx returns a context whose logger writes into the returned sink.
func observedCtx(t *testing.T) (context.Context, *observer.ObservedLogs) {
	t.Helper()

	core, logs := observer.New(zapcore.DebugLevel)

	return xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zap.New(core))), logs
}

// signIn mints a session from the given client and returns its pair.
func signIn(ctx context.Context, t *testing.T, srv *Service, mocks *serviceMocks, clientIP, userAgent string) *entity.TokenPair {
	t.Helper()

	exchangeIDTokenMock(mocks, 1)

	pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
		Provider:  entity.AuthMethodGoogle,
		IDToken:   "id-token",
		ClientIP:  clientIP,
		UserAgent: userAgent,
	})
	require.NoError(t, err)

	return pair
}

// TestRefresh_RecordsTheCurrentClient pins that every row carries the address
// of the request that minted it -- the sign-in for the first, the refresh for
// each rotation -- rather than a value copied down the chain.
func TestRefresh_RecordsTheCurrentClient(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv, mocks := initService(t)

	pair := signIn(ctx, t, srv, mocks, "203.0.113.7", "Firefox/131")

	first, err := srv.tokenSrv.GetRefreshToken(ctx, pair.RefreshToken)
	require.NoError(t, err)
	require.Equal(t, "203.0.113.7", first.ClientIP)

	rotated, err := srv.Refresh(ctx, pair.RefreshToken, "198.51.100.9")
	require.NoError(t, err)

	second, err := srv.tokenSrv.GetRefreshToken(ctx, rotated.RefreshToken)
	require.NoError(t, err)
	require.Equal(t, "198.51.100.9", second.ClientIP, "the rotated row must carry the refreshing request's address")

	prev, err := srv.tokenSrv.GetRefreshToken(ctx, pair.RefreshToken)
	require.NoError(t, err)
	require.Equal(t, "203.0.113.7", prev.ClientIP, "rotation must not rewrite the old row's record")
}

// TestRefresh_LogsAChangeOfAddress pins the line that replaced the binding:
// a rotation from a new address goes ahead and leaves one info record naming
// both addresses, a rotation from the same address leaves none, and neither
// does a grace-window refresh, which rotates nothing.
func TestRefresh_LogsAChangeOfAddress(t *testing.T) {
	t.Parallel()

	srv, mocks := initService(t)
	srv.cfg.RefreshTokenGracePeriod = 30 * time.Second
	pair := signIn(context.Background(), t, srv, mocks, "203.0.113.7", "Firefox/131")

	sameCtx, sameLogs := observedCtx(t)
	same, err := srv.Refresh(sameCtx, pair.RefreshToken, "203.0.113.7")
	require.NoError(t, err)
	require.Empty(t, sameLogs.FilterMessage("session refreshed from a new address").All())

	movedCtx, movedLogs := observedCtx(t)
	moved, err := srv.Refresh(movedCtx, same.RefreshToken, "198.51.100.9")
	require.NoError(t, err)
	require.NotEmpty(t, moved.RefreshToken, "a new address must not cost the session")

	lines := movedLogs.FilterMessage("session refreshed from a new address").All()
	require.Len(t, lines, 1)
	require.Equal(t, zapcore.InfoLevel, lines[0].Level)
	fields := lines[0].ContextMap()
	require.Equal(t, "203.0.113.7", fields["prev_ip"])
	require.Equal(t, "198.51.100.9", fields["ip"])
	require.Equal(t, pair.SessionID.String(), fmt.Sprint(fields["family"]))
	require.NotContains(t, fields, "ua_changed", "the User-Agent is not compared any more")

	graceCtx, graceLogs := observedCtx(t)
	graced, err := srv.Refresh(graceCtx, same.RefreshToken, "192.0.2.1")
	require.NoError(t, err)
	require.Empty(t, graced.RefreshToken, "the second refresh must have taken the grace path")
	require.Empty(t, graceLogs.FilterMessage("session refreshed from a new address").All())
}

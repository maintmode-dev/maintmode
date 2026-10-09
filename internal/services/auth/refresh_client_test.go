package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
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

	rotated, err := srv.Refresh(ctx, pair.RefreshToken, "198.51.100.9", "Firefox/131")
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
	same, err := srv.Refresh(sameCtx, pair.RefreshToken, "203.0.113.7", "Firefox/131")
	require.NoError(t, err)
	require.Empty(t, sameLogs.FilterMessage("session refreshed from a new address").All())

	movedCtx, movedLogs := observedCtx(t)
	moved, err := srv.Refresh(movedCtx, same.RefreshToken, "198.51.100.9", "Firefox/131")
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
	graced, err := srv.Refresh(graceCtx, same.RefreshToken, "192.0.2.1", "curl/8.4.0")
	require.NoError(t, err)
	require.Empty(t, graced.RefreshToken, "the second refresh must have taken the grace path")
	require.Empty(t, graceLogs.FilterMessage("session refreshed from a new address").All())
}

// TestRefresh_ReuseDetectionIsAudited pins the session.revoked row: replaying a
// rotated-out token past its grace window revokes the session AND records it
// against the owner, with the replaying request's address and User-Agent. Every
// replay is recorded, a repeated one included: each is a fresh sign that the
// token is in someone else's hands.
func TestRefresh_ReuseDetectionIsAudited(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	srv, mocks := initService(t)
	rec := newRecordingAuditPublisher()
	srv.auditPublisher = rec

	pair := signIn(ctx, t, srv, mocks, "203.0.113.7", "Firefox/131")

	rotated, err := srv.Refresh(ctx, pair.RefreshToken, "203.0.113.7", "Firefox/131")
	require.NoError(t, err)

	expireGraceWindow(ctx, t, srv, pair.RefreshToken)

	const replayerIP, replayerUA = "198.51.100.9", "curl/8.4.0"
	replayCtx, replayLogs := observedCtx(t)
	_, err = srv.Refresh(replayCtx, pair.RefreshToken, replayerIP, replayerUA)
	require.ErrorIs(t, err, apperr.ErrTokenReuse)
	reuse := requireLoggedAt(t, replayLogs, "token reuse detected", zapcore.WarnLevel)
	require.Contains(t, reuse.ContextMap(), "grace_period")

	live, err := srv.tokenSrv.GetRefreshToken(ctx, rotated.RefreshToken)
	require.NoError(t, err)
	require.True(t, live.Revoked, "reuse must revoke the whole family")

	revocations := sessionRevokedRows(rec.actions())
	require.Len(t, revocations, 1)

	row := revocations[0]
	require.Equal(t, live.UserID, row.User.ID)
	require.NotEmpty(t, row.User.Email, "the actor is the session's resolved owner")
	require.Equal(t, replayerIP, row.Meta.IP)
	require.Equal(t, replayerUA, row.Meta.UserAgent)
	require.Equal(t, pair.SessionID.String(), row.Meta.SessionID)
	require.Equal(t, entity.AuditRevokeReasonTokenReuse, row.Meta.RevokeReason)

	_, err = srv.Refresh(ctx, pair.RefreshToken, replayerIP, replayerUA)
	require.ErrorIs(t, err, apperr.ErrTokenReuse)
	require.Len(t, sessionRevokedRows(rec.actions()), 2, "a repeated replay is recorded again")
}

// expireGraceWindow moves a rotated token's grace window into the past, so the
// next presentation of it counts as reuse rather than a racing tab.
func expireGraceWindow(ctx context.Context, t *testing.T, srv *Service, raw string) {
	t.Helper()

	rt, err := srv.tokenSrv.GetRefreshToken(ctx, raw)
	require.NoError(t, err)
	require.True(t, rt.Revoked)

	rt.GraceTTL = lo.ToPtr(rt.GraceTTL.Add(-srv.cfg.RefreshTokenGracePeriod - time.Second))
	require.NoError(t, srv.tokenSrv.UpdateRefreshToken(ctx, rt))
}

func sessionRevokedRows(published []audit.Action) []audit.SessionRevoked {
	var rows []audit.SessionRevoked
	for _, action := range published {
		if row, ok := action.(audit.SessionRevoked); ok {
			rows = append(rows, row)
		}
	}

	return rows
}

// requireLoggedAt asserts msg was logged exactly once, at level, and returns
// the line.
func requireLoggedAt(t *testing.T, logs *observer.ObservedLogs, msg string, level zapcore.Level) observer.LoggedEntry {
	t.Helper()

	lines := logs.FilterMessage(msg).All()
	require.Len(t, lines, 1)
	require.Equal(t, level, lines[0].Level)

	return lines[0]
}

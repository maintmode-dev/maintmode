package auth

import (
	"context"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// A refresh that outlived its lock still holds the row it read before another
// refresh rotated it. Rotating from that stale read must not give the token a
// second successor.
func TestRotateRefreshToken_StaleReadDoesNotRotateTwice(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, mocks := initService(t)
	exchangeIDTokenMock(mocks, 1)

	pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
		Provider: entity.AuthMethodGoogle,
		IDToken:  "id-token",
		ClientIP: "10.0.0.1",
	})
	require.NoError(t, err)

	stale, err := srv.tokenSrv.GetRefreshTokenByHash(ctx, xhash.HashSha256([]byte(pair.RefreshToken)))
	require.NoError(t, err)

	first, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
	require.NoError(t, err)

	obsCtx, logs := observedCtx(t)
	second, err := srv.rotateRefreshToken(obsCtx, stale, xtime.UTCNow(), "10.0.0.1")
	require.ErrorIs(t, err, apperr.ErrLockBusy)
	require.Nil(t, second)
	// Losing the race is expected under load, not a failure to page on.
	requireLoggedAt(t, logs, "refresh token rotated concurrently", zapcore.WarnLevel)
	require.Zero(t, logs.FilterMessage("failed to rotate refresh token").Len())

	rotated, err := srv.tokenSrv.GetRefreshTokenByHash(ctx, stale.Token)
	require.NoError(t, err)
	require.Equal(t, xhash.HashSha256([]byte(first.RefreshToken)), *rotated.ReplacedBy,
		"the first rotation's successor must stand")

	var rows int
	require.NoError(t, db.GetContext(ctx, &rows,
		`SELECT count(*) FROM refresh_tokens WHERE family = $1`, stale.Family))
	require.Equal(t, 2, rows, "the lost rotation must not have saved a successor")
}

// The same race while it is happening: another transaction holds the token's
// row when Refresh tries to rotate it, and rotates it first. Refresh must wait
// for it, see the token already rotated, and give up -- not overwrite it.
func TestRefresh_RotationBlockedByAConcurrentOneGivesUp(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	srv, mocks := initService(t)
	exchangeIDTokenMock(mocks, 1)

	pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
		Provider: entity.AuthMethodGoogle,
		IDToken:  "id-token",
		ClientIP: "10.0.0.1",
	})
	require.NoError(t, err)
	tokenHash := xhash.HashSha256([]byte(pair.RefreshToken))

	// The other rotation: it holds the row until it commits.
	other, err := db.BeginTxx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = other.Rollback() })

	var otherPID int
	require.NoError(t, other.GetContext(ctx, &otherPID, `SELECT pg_backend_pid()`))
	_, err = other.ExecContext(ctx, `SELECT 1 FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`, tokenHash)
	require.NoError(t, err)

	refreshed := make(chan error, 1)
	go func() {
		_, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
		refreshed <- err
	}()

	// Wait on the database, not a timer: Refresh's rotation is queued behind
	// the other transaction's row lock.
	require.Eventually(t, func() bool {
		var waiting int
		err := db.GetContext(ctx, &waiting,
			`SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))`, otherPID)
		return err == nil && waiting == 1
	}, 10*time.Second, 10*time.Millisecond)

	const otherSuccessor = "the-other-rotation's-successor"
	_, err = other.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked = true, replaced_by = $2 WHERE token_hash = $1`,
		tokenHash, otherSuccessor)
	require.NoError(t, err)
	require.NoError(t, other.Commit())

	require.ErrorIs(t, <-refreshed, apperr.ErrLockBusy)

	rotated, err := srv.tokenSrv.GetRefreshTokenByHash(ctx, tokenHash)
	require.NoError(t, err)
	require.Equal(t, otherSuccessor, *rotated.ReplacedBy, "the other rotation's successor must stand")

	var rows int
	require.NoError(t, db.GetContext(ctx, &rows,
		`SELECT count(*) FROM refresh_tokens WHERE family = $1`, rotated.Family))
	require.Equal(t, 1, rows, "the losing rotation must not have saved a successor")
}

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// A revocation that overlaps a rotation must still end the session. The
// rotation inserts the successor; a revocation whose statement started before
// that insert committed never sees the successor, and the session outlives its
// own revocation.
//
// The session is rotated once first, so it has a predecessor still inside its
// grace window. The next rotation is paused on its predecessor's row while the
// revocation starts, then let through: the order the two commit in is the one
// that lost the successor. A logout presents the grace-window predecessor, so
// its own row lock does not happen to wait for the rotation.
func TestRevocationOverlappingARotationEndsTheSession(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	revokers := map[string]func(srv *Service, rt *entity.RefreshToken, raw string) error{
		"by family": func(srv *Service, rt *entity.RefreshToken, _ string) error {
			return srv.tokenSrv.RevokeRefreshTokenByFamily(ctx, rt.UserID, rt.Family)
		},
		"by user": func(srv *Service, rt *entity.RefreshToken, _ string) error {
			return srv.tokenSrv.RevokeRefreshTokenByUserID(ctx, rt.UserID)
		},
		"by user except another session": func(srv *Service, rt *entity.RefreshToken, _ string) error {
			return srv.tokenSrv.RevokeRefreshTokenByUserIDExceptFamily(ctx, rt.UserID, uuid.New())
		},
		"by a presented predecessor": func(srv *Service, rt *entity.RefreshToken, predecessor string) error {
			_, err := srv.tokenSrv.RevokeFamilyByRefreshToken(ctx, predecessor, &entity.AccessClaims{
				RegisteredClaims: jwt.RegisteredClaims{Subject: rt.UserID.String()},
			})
			return err
		},
	}

	for name, revoke := range revokers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			srv, _ := initService(t)

			pair, err := signInAs(ctx, srv, newTestOAuthUser(), &entity.AuditMetadata{IP: "10.0.0.1"})
			require.NoError(t, err)
			current, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.1", "")
			require.NoError(t, err)
			rt, err := srv.tokenSrv.GetRefreshTokenByHash(ctx, xhash.HashSha256([]byte(current.RefreshToken)))
			require.NoError(t, err)

			// Hold the token's row: the rotation queues behind it.
			pause, err := db.BeginTxx(ctx, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = pause.Rollback() })
			var pausePID int64
			require.NoError(t, pause.GetContext(ctx, &pausePID, `SELECT pg_backend_pid()`))
			_, err = pause.ExecContext(ctx, `SELECT 1 FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`, rt.Token)
			require.NoError(t, err)

			rotated := make(chan error, 1)
			go func() {
				_, err := srv.Refresh(ctx, current.RefreshToken, "10.0.0.1", "")
				rotated <- err
			}()
			rotationPID := waitForBlockedBy(ctx, t, pausePID)

			revoked := make(chan error, 1)
			go func() { revoked <- revoke(srv, rt, pair.RefreshToken) }()
			waitForBlockedByAny(ctx, t, rotationPID, pausePID, rotationPID)

			require.NoError(t, pause.Rollback())
			require.NoError(t, <-rotated)
			require.NoError(t, <-revoked)

			var live int
			require.NoError(t, db.GetContext(ctx, &live,
				`SELECT count(*) FROM refresh_tokens WHERE family = $1 AND NOT revoked`, rt.Family))
			require.Zero(t, live, "the revoked session must have no live refresh token left")
		})
	}
}

// waitForBlockedBy waits until a backend is blocked by blocker and returns it.
func waitForBlockedBy(ctx context.Context, t *testing.T, blocker int64) int64 {
	t.Helper()

	var blocked int64
	require.Eventually(t, func() bool {
		err := db.GetContext(ctx, &blocked,
			`SELECT pid FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)) LIMIT 1`, blocker)
		return err == nil
	}, 10*time.Second, 10*time.Millisecond)

	return blocked
}

// waitForBlockedByAny waits until a backend other than exclude is blocked by
// any of blockers.
func waitForBlockedByAny(ctx context.Context, t *testing.T, exclude int64, blockers ...int64) {
	t.Helper()

	require.Eventually(t, func() bool {
		var waiting int
		err := db.GetContext(ctx, &waiting, `
			SELECT count(*) FROM pg_stat_activity
			WHERE pid <> $1 AND pg_blocking_pids(pid) && $2::int[]`,
			exclude, pq.Array(blockers))
		return err == nil && waiting > 0
	}, 10*time.Second, 10*time.Millisecond)
}

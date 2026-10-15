package auth

import (
	"context"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhash"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestSessionExpiresAt(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	startedAt := func(t *testing.T, srv *Service, refreshToken string) time.Time {
		t.Helper()

		rt, err := srv.tokenSrv.GetRefreshTokenByHash(ctx, xhash.HashSha256([]byte(refreshToken)))
		require.NoError(t, err)

		return rt.SessionStartedAt
	}

	t.Run("sign-in plus the maximum lifetime, unmoved by rotation and grace", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)
		srv.cfg.RefreshTokenGracePeriod = 30 * time.Second

		signedIn, err := signInAs(ctx, srv, newTestOAuthUser(), &entity.AuditMetadata{IP: "10.0.0.1"})
		require.NoError(t, err)

		want := startedAt(t, srv, signedIn.RefreshToken).Add(srv.cfg.SessionMaxLifetime).Truncate(time.Second)
		require.Equal(t, want, signedIn.SessionExpiresAt)
		require.Equal(t, time.UTC, signedIn.SessionExpiresAt.Location())

		rotated, err := srv.Refresh(ctx, signedIn.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)
		require.NotEmpty(t, rotated.RefreshToken)
		require.Equal(t, want, rotated.SessionExpiresAt, "rotation must not move the session's end")

		replayed, err := srv.Refresh(ctx, signedIn.RefreshToken, "10.0.0.1", "")
		require.NoError(t, err)
		require.Empty(t, replayed.RefreshToken, "the replay must land in the grace window")
		require.Equal(t, want, replayed.SessionExpiresAt, "the grace answer reports the same end")
	})

	// The break-glass day must hold on every answer, not only at sign-in: a
	// refresh reporting the configured lifetime would promise the BFF a cookie
	// that outlives the session by weeks.
	t.Run("break-glass ends a day after the sign-in, unmoved by rotation and grace", func(t *testing.T) {
		t.Parallel()

		breakGlass := newTestBreakGlass("the-break-glass-" + xuuid.NewString())
		srv, _ := initServiceWithBreakGlass(t, breakGlass, nil)
		srv.cfg.RefreshTokenGracePeriod = 30 * time.Second
		pair, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{
			Password: breakGlass.password, ClientIP: "10.0.0.7",
		})
		require.NoError(t, err)

		want := startedAt(t, srv, pair.RefreshToken).Add(24 * time.Hour).Truncate(time.Second)
		require.Equal(t, want, pair.SessionExpiresAt)

		rotated, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.7", "")
		require.NoError(t, err)
		require.NotEmpty(t, rotated.RefreshToken)
		require.Equal(t, want, rotated.SessionExpiresAt, "rotation must keep the break-glass day")

		replayed, err := srv.Refresh(ctx, pair.RefreshToken, "10.0.0.7", "")
		require.NoError(t, err)
		require.Empty(t, replayed.RefreshToken, "the replay must land in the grace window")
		require.Equal(t, want, replayed.SessionExpiresAt, "the grace answer must keep the break-glass day")
	})
}

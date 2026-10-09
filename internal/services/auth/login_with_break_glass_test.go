package auth

import (
	"context"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// breakGlassAccount is the account a break-glass sign-in reached.
func breakGlassAccount(ctx context.Context, t *testing.T, srv *Service, breakGlass *testBreakGlass) *entity.User {
	t.Helper()

	account, err := srv.usersSrv.GetByEmail(ctx, breakGlass.email)
	require.NoError(t, err)

	return account
}

func TestLoginWithBreakGlass(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("the password alone signs in to an admin account of its own", func(t *testing.T) {
		t.Parallel()

		breakGlass := newTestBreakGlass("the-break-glass-" + xuuid.NewString())
		srv, _ := initServiceWithBreakGlass(t, breakGlass, nil)

		pair, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{
			Password: breakGlass.password, ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		access, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.Contains(t, access.UserRoles, entity.RoleAdmin)

		account := breakGlassAccount(ctx, t, srv, breakGlass)
		require.Equal(t, breakGlass.email, account.Email)
		require.Equal(t, "Break-glass admin", account.Name)
	})

	t.Run("a wrong password is refused and audited as a break-glass attempt", func(t *testing.T) {
		t.Parallel()

		srv, _ := initServiceWithBreakGlass(t, newTestBreakGlass("the-break-glass-"+xuuid.NewString()), nil)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{
			Password: "not-the-break-glass-password", ClientIP: "10.0.0.2",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidCredentials)

		actions := publisher.actions()
		require.Len(t, actions, 1)
		failed, ok := actions[0].(audit.LoginFailed)
		require.True(t, ok, "expected a login failure, got %T", actions[0])
		require.Equal(t, entity.AuditFailureInvalidCredentials, failed.Meta.FailureReason)
		require.Equal(t, entity.AuditLoginMethodBootstrap, failed.Meta.LoginMethod)
		require.Equal(t, "10.0.0.2", failed.Meta.IP)

		payload, renderErr := audit.NewRenderer().Render(failed)
		require.NoError(t, renderErr, "the actor of a pre-identification failure must render")
		require.Equal(t, entity.AuditActionLoginFailed, payload.Action)
	})

	t.Run("a success is audited as break-glass with its session", func(t *testing.T) {
		t.Parallel()

		breakGlass := newTestBreakGlass("the-break-glass-" + xuuid.NewString())
		srv, _ := initServiceWithBreakGlass(t, breakGlass, nil)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		pair, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{
			Password: breakGlass.password, ClientIP: "10.0.0.3", UserAgent: "curl/8",
		})
		require.NoError(t, err)

		actions := publisher.actions()
		require.NotEmpty(t, actions)
		success, ok := actions[len(actions)-1].(audit.LoginSuccess)
		require.True(t, ok, "the last event must be a login success, got %T", actions[len(actions)-1])
		require.Equal(t, entity.AuditLoginMethodBootstrap, success.Meta.LoginMethod)
		require.Equal(t, pair.SessionID.String(), success.Meta.SessionID)
		require.Equal(t, "curl/8", success.Meta.UserAgent)
	})

	// Blocking the account is how an admin switches break-glass off without
	// touching the configuration.
	t.Run("a blocked break-glass account is refused", func(t *testing.T) {
		t.Parallel()

		breakGlass := newTestBreakGlass("the-break-glass-" + xuuid.NewString())
		srv, _ := initServiceWithBreakGlass(t, breakGlass, nil)
		_, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{Password: breakGlass.password})
		require.NoError(t, err)

		require.NoError(t, srv.usersSrv.BlockUser(ctx, &entity.BlockUserCmd{
			UserID: breakGlassAccount(ctx, t, srv, breakGlass).ID,
			Actor:  makePasswordUser(ctx, t, nil, ""),
		}))

		_, err = srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{Password: breakGlass.password})
		require.ErrorIs(t, err, apperr.ErrUserBlocked)
	})

	// Its way in is the configured password; a personal one would outlive a
	// change of that password and keep the access the change revokes.
	t.Run("the break-glass account cannot set a personal password", func(t *testing.T) {
		t.Parallel()

		breakGlass := newTestBreakGlass("the-break-glass-" + xuuid.NewString())
		srv, _ := initServiceWithBreakGlass(t, breakGlass, nil)
		_, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{Password: breakGlass.password})
		require.NoError(t, err)

		err = srv.ChangePassword(ctx, &entity.ChangePasswordCmd{
			UserID:      breakGlassAccount(ctx, t, srv, breakGlass).ID,
			NewPassword: "a-personal-password-" + xuuid.NewString(),
			ClientIP:    "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrBreakGlassPersonalSignIn)
	})
}

// TestBreakGlassSessionIsShort pins the break-glass session's lifetimes: its
// access token lasts five minutes and the session ends a day after the sign-in,
// however often it is refreshed. That is what bounds the access of whoever once
// held the break-glass password -- there is no revocation on a password change.
func TestBreakGlassSessionIsShort(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))
	const clientIP = "10.0.0.7"

	signIn := func(t *testing.T) (*Service, *entity.TokenPair) {
		t.Helper()

		breakGlass := newTestBreakGlass("the-break-glass-" + xuuid.NewString())
		srv, _ := initServiceWithBreakGlass(t, breakGlass, nil)
		pair, err := srv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{
			Password: breakGlass.password, ClientIP: clientIP,
		})
		require.NoError(t, err)

		return srv, pair
	}
	accessLifetime := func(t *testing.T, srv *Service, pair *entity.TokenPair) time.Duration {
		t.Helper()

		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)

		return claims.ExpiresAt.Sub(claims.IssuedAt.Time)
	}

	t.Run("the access token lasts five minutes", func(t *testing.T) {
		t.Parallel()

		srv, pair := signIn(t)

		require.Equal(t, 300, pair.ExpiresIn)
		require.Equal(t, 5*time.Minute, accessLifetime(t, srv, pair))
	})

	t.Run("a refresh issues a short access token again", func(t *testing.T) {
		t.Parallel()

		srv, pair := signIn(t)
		// A real grace window, so the replay below takes the grace branch too.
		srv.cfg.RefreshTokenGracePeriod = 30 * time.Second

		refreshed, err := srv.Refresh(ctx, pair.RefreshToken, clientIP, "")
		require.NoError(t, err)
		require.Equal(t, 300, refreshed.ExpiresIn)
		require.Equal(t, 5*time.Minute, accessLifetime(t, srv, refreshed))

		replayed, err := srv.Refresh(ctx, pair.RefreshToken, clientIP, "")
		require.NoError(t, err)
		require.Empty(t, replayed.RefreshToken, "the replay must land in the grace window")
		require.Equal(t, 300, replayed.ExpiresIn)
		require.Equal(t, 5*time.Minute, accessLifetime(t, srv, replayed))
	})

	t.Run("refreshing does not carry the session past a day after the sign-in", func(t *testing.T) {
		t.Parallel()

		srv, pair := signIn(t)
		_, err := db.ExecContext(ctx,
			`UPDATE refresh_tokens SET session_started_at = now() - interval '25 hours' WHERE family = $1`,
			pair.SessionID)
		require.NoError(t, err)

		_, err = srv.Refresh(ctx, pair.RefreshToken, clientIP, "")
		require.ErrorIs(t, err, apperr.ErrTokenExpired)
	})
}

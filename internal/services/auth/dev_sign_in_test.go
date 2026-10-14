package auth

import (
	"context"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestDevSignIn(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	// Invite-only is the shape that matters: on an open-signup stand a fresh
	// identity is created anyway, so a DevSignIn that forgot AllowCreate would
	// pass there and be refused on the API test stack.
	t.Run("creates a user with the requested roles despite invite-only signup", func(t *testing.T) {
		t.Parallel()

		srv, _ := initInviteOnlyService(t)

		pair, err := srv.DevSignIn(ctx, &entity.DevSignInCmd{
			Roles:    []entity.Role{entity.RoleEditor, entity.RoleReviewer},
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.Contains(t, claims.UserRoles, entity.RoleEditor)
		require.Contains(t, claims.UserRoles, entity.RoleReviewer)
		require.Contains(t, claims.UserRoles, entity.RoleGuest, "requested roles come on top of the defaults")
	})

	// AssignRoles filters against the roles the user already holds, not within
	// the request, so a repeated role would be stored twice.
	t.Run("a repeated role is granted once", func(t *testing.T) {
		t.Parallel()

		srv, _ := initInviteOnlyService(t)

		pair, err := srv.DevSignIn(ctx, &entity.DevSignInCmd{
			Roles:    []entity.Role{entity.RoleEditor, entity.RoleEditor},
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)

		var editors int
		for _, role := range claims.UserRoles {
			if role == entity.RoleEditor {
				editors++
			}
		}
		require.Equal(t, 1, editors, "roles: %v", claims.UserRoles)
	})

	// The trail is how an operator tells who signed in from where; the caller's
	// address and agent reach it only through DevSignIn's own mapping.
	t.Run("audits a successful login with the caller's address and agent", func(t *testing.T) {
		t.Parallel()

		srv, _ := initInviteOnlyService(t)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.DevSignIn(ctx, &entity.DevSignInCmd{
			Roles:     []entity.Role{entity.RoleEditor},
			ClientIP:  "198.51.100.7",
			UserAgent: "k6/0.50",
		})
		require.NoError(t, err)

		success, ok := onlyAction(t, publisher).(audit.LoginSuccess)
		require.True(t, ok, "a dev sign-in must be audited as a successful login")
		require.Equal(t, "198.51.100.7", success.Meta.IP)
		require.Equal(t, "k6/0.50", success.Meta.UserAgent)
	})

	// The user service rejects an unknown role as well, but only after the
	// sign-in has filed a login.failed for an invented identity nobody can look
	// up -- noise in the trail where a run of failures reads as an attack.
	t.Run("an unknown role is refused without auditing a failed login", func(t *testing.T) {
		t.Parallel()

		srv, _ := initInviteOnlyService(t)
		publisher := newRecordingAuditPublisher()
		srv.auditPublisher = publisher

		_, err := srv.DevSignIn(ctx, &entity.DevSignInCmd{
			Roles:    []entity.Role{entity.RoleEditor, "superuser"},
			ClientIP: "10.0.0.1",
		})
		require.ErrorIs(t, err, apperr.ErrInvalidRole)
		require.Empty(t, publisher.actions())
	})
}

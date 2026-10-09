package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestIntrospect(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("active", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		oauthUser := exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		instrospectResp, err := srv.Introspect(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.True(t, instrospectResp.Active)
		require.Equal(t, oauthUser.Email, instrospectResp.Email)

		// Verify access token claims
		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.NotNil(t, claims)
		require.Equal(t, claims.ID, instrospectResp.JTI)
		require.Equal(t, claims.Subject, instrospectResp.Subject)
		require.Equal(t, claims.UserEmail, instrospectResp.Email)
		require.Equal(t, claims.ExpiresAt.Unix(), instrospectResp.Exp)
		require.Equal(t, claims.UserRoles, instrospectResp.Roles)
	})

	t.Run("blacklisted", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		err = srv.Logout(ctx, &entity.TokenPair{
			AccessToken:  pair.AccessToken,
			RefreshToken: pair.RefreshToken,
		})
		require.NoError(t, err)

		resp, err := srv.Introspect(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.False(t, resp.Active)
	})

	t.Run("blocked user", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)

		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		// Token is active before the block.
		resp, err := srv.Introspect(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.True(t, resp.Active)

		// Block the token's subject; a live access token must become inactive on
		// the next introspect, not only after expiry.
		userID, err := uuid.Parse(resp.Subject)
		require.NoError(t, err)
		require.NoError(t, srv.usersSrv.BlockUser(ctx, &entity.BlockUserCmd{
			Actor:  &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleAdmin}},
			UserID: userID,
		}))

		resp, err = srv.Introspect(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.False(t, resp.Active)
	})

	t.Run("invalid token", func(t *testing.T) {
		srv, _ := initService(t)

		resp, err := srv.Introspect(ctx, "garbage-token")
		require.NoError(t, err)
		require.False(t, resp.Active)
	})

	t.Run("expired token", func(t *testing.T) {
		srv, _ := initService(t)

		// Manually craft an expired but properly signed token
		token, err := srv.tokenSrv.IssueAccessToken(ctx, time.Millisecond, &entity.User{
			ID:    uuid.New(),
			Email: "some@email.com",
			Roles: entity.DefaultRoles,
		}, uuid.New())
		require.NoError(t, err)

		resp, err := srv.Introspect(ctx, token)
		require.NoError(t, err)
		require.False(t, resp.Active)
	})
}

// TestEnsureActiveToken pins the security-critical translation the API
// middleware relies on: an active token yields nil, while any inactive token
// (revoked by logout, or belonging to a blocked user) yields
// ErrInvalidAccessToken so RequireActiveToken rejects the request. This is the
// direct unit coverage that used to live in the deleted authlocal adapter test.
func TestEnsureActiveToken(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("active token returns nil", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		_, err = srv.EnsureActiveToken(ctx, pair.AccessToken)
		require.NoError(t, err)
	})

	t.Run("returns stored roles, not token roles", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)
		claims, err := srv.tokenSrv.VerifyAccessToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.NotContains(t, claims.UserRoles, entity.RoleEditor)

		// A role change after issuance must be what a write is authorized on.
		userID, err := uuid.Parse(claims.Subject)
		require.NoError(t, err)
		_, err = srv.usersSrv.AssignRoles(ctx, &entity.AssignRolesCmd{
			Actor:  &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleAdmin}},
			UserID: userID,
			Roles:  []entity.Role{entity.RoleEditor},
		})
		require.NoError(t, err)

		roles, err := srv.EnsureActiveToken(ctx, pair.AccessToken)
		require.NoError(t, err)
		require.Contains(t, roles, entity.RoleEditor)
	})

	t.Run("blacklisted token is rejected", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)

		require.NoError(t, srv.Logout(ctx, &entity.TokenPair{
			AccessToken:  pair.AccessToken,
			RefreshToken: pair.RefreshToken,
		}))

		_, err = srv.EnsureActiveToken(ctx, pair.AccessToken)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("blocked-user token is rejected", func(t *testing.T) {
		t.Parallel()

		srv, mocks := initService(t)
		exchangeIDTokenMock(mocks, 1)

		pair, err := srv.ExchangeIDToken(ctx, &entity.ExchangeIDTokenCmd{
			Provider: entity.AuthMethodGoogle,
			IDToken:  "id-token",
			ClientIP: "10.0.0.1",
		})
		require.NoError(t, err)
		_, err = srv.EnsureActiveToken(ctx, pair.AccessToken)
		require.NoError(t, err)

		report, err := srv.Introspect(ctx, pair.AccessToken)
		require.NoError(t, err)
		userID, err := uuid.Parse(report.Subject)
		require.NoError(t, err)
		require.NoError(t, srv.usersSrv.BlockUser(ctx, &entity.BlockUserCmd{
			Actor:  &entity.User{ID: uuid.New(), Roles: []entity.Role{entity.RoleAdmin}},
			UserID: userID,
		}))

		_, err = srv.EnsureActiveToken(ctx, pair.AccessToken)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("invalid token is rejected", func(t *testing.T) {
		t.Parallel()

		srv, _ := initService(t)
		_, err := srv.EnsureActiveToken(ctx, "garbage-token")
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})
}

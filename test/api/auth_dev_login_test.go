//go:build api

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
)

// The suite provisions every user through this route, so the whole run already
// leans on it; this pins the parts the provisioning helpers do not look at.
func TestAuthAPI_DevLogin(t *testing.T) {
	ctx := ctxWithLogger(context.Background(), t)

	t.Run("grants the requested roles on top of the defaults", func(t *testing.T) {
		resp, err := setupAuthTestClient().PostApiV1LoginDevWithResponse(ctx, devLoginBody(entity.RoleEditor))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
		require.NotNil(t, resp.JSON200)
		require.NotEmpty(t, lo.FromPtr(resp.JSON200.RefreshToken))

		var claims entity.AccessClaims
		_, _, err = jwt.NewParser().ParseUnverified(lo.FromPtr(resp.JSON200.AccessToken), &claims)
		require.NoError(t, err)
		require.Contains(t, claims.UserRoles, entity.RoleEditor)
		require.Contains(t, claims.UserRoles, entity.RoleGuest)
	})

	// The identity is invented server-side, so nothing the caller sends can land
	// two sign-ins in one account.
	t.Run("every call is a new user", func(t *testing.T) {
		_, first := provisionUser(ctx, t)
		_, second := provisionUser(ctx, t)
		require.NotEqual(t, first, second)
	})

	t.Run("an unknown role is refused", func(t *testing.T) {
		resp, err := setupAuthTestClient().PostApiV1LoginDevWithResponse(ctx,
			authclient.PostApiV1LoginDevJSONRequestBody{Roles: &[]string{"superuser"}})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode(), "unexpected status: %s", resp.Body)
	})
}

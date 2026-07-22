package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/require"

	testjsonudils "github.com/ruko1202/maintmode/test/utils/json"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// TestExchangeTestRoles covers how the exchange handler folds the roles the
// X-Test-Roles middleware left in the echo context into the login. Parsing and
// the 400-on-unknown-role live in the middleware test; here the roles are
// seeded straight into the context, standing in for the middleware.
func TestExchangeTestRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	impl := initImpl(t)

	// Pin the installation past bootstrap before the subtests run: an active
	// admin must exist, otherwise the roleless user below would be promoted to
	// the first admin and the guest assertion would flap on a fresh database.
	rec := doExchange(t, impl, entity.RoleAdmin)
	require.Equal(t, http.StatusOK, rec.Code)

	t.Run("roles from context are granted", func(t *testing.T) {
		t.Parallel()

		rec := doExchange(t, impl, entity.RoleAdmin, entity.RoleReviewer)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apiauthmodels.TokenPairResponse](t, rec.Body)
		claims, err := impl.tokenSrv.VerifyAccessToken(ctx, resp.AccessToken)
		require.NoError(t, err)
		require.Subset(t, claims.UserRoles, []entity.Role{entity.RoleAdmin, entity.RoleReviewer})
	})

	t.Run("no roles in context: user stays guest", func(t *testing.T) {
		t.Parallel()

		// Without the middleware there are no roles in context; the user is
		// created via open signup as a plain guest.
		require.True(t, cfg.Auth.AllowOpenSignup, "test config must enable auth.allow_open_signup")

		rec := doExchange(t, impl)
		require.Equal(t, http.StatusOK, rec.Code)

		resp := testjsonudils.JSONToAny[apiauthmodels.TokenPairResponse](t, rec.Body)
		claims, err := impl.tokenSrv.VerifyAccessToken(ctx, resp.AccessToken)
		require.NoError(t, err)
		require.NotContains(t, claims.UserRoles, entity.RoleAdmin)
		require.Contains(t, claims.UserRoles, entity.RoleGuest)
	})
}

// doExchange posts a stub ID token to the exchange handler, seeding the given
// roles into the echo context the way the X-Test-Roles middleware would.
func doExchange(t *testing.T, impl *Implementation, roles ...entity.Role) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/auth/exchange/google", http.NoBody)
	c, rec := echotest.ContextConfig{
		Request: request,
		JSONBody: testjsonudils.AnyToJSONBytes(t, apiauthmodels.ExchangeIDTokenRequest{
			IDToken: "stub-id-token",
		}),
	}.ToContextRecorder(t)

	seedTestRoles(c, roles)

	require.NoError(t, impl.ExchangeGoogleToken(c))
	return rec
}

// seedTestRoles mimics the X-Test-Roles middleware by placing roles in context.
func seedTestRoles(c *echo.Context, roles []entity.Role) {
	if len(roles) > 0 {
		xecho.TestRolesToEchoCtx(c, roles)
	}
}

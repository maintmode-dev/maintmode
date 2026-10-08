package middlewares

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// ActiveTokenChecker verifies an access token is still active (not revoked by
// logout, not belonging to a blocked user) and returns the subject's CURRENT
// roles from the user store. Declared consumer-side; satisfied in-process by
// *auth.Service.EnsureActiveToken.
type ActiveTokenChecker interface {
	EnsureActiveToken(ctx context.Context, tokenString string) ([]entity.Role, error)
}

// RequireActiveToken re-checks the token against server-side state on every
// request that can change something: it must not be revoked (logout /
// logout-all), its user must not be blocked, and the roles RBAC sees are the
// user's stored roles rather than the ones baked into the JWT. It must run
// AFTER RequireAccessToken (which validated the signature and put the user in
// the context) and BEFORE RequireScenario (which reads the roles it replaces).
//
// Without it a blocked or demoted user keeps every write their token's claims
// allow until the token expires — enough to unblock themselves or re-grant a
// revoked role.
//
// Safe methods (GET/HEAD/OPTIONS) pass through on local JWT validation: reads
// are the hot path, and their exposure ends with the access-token TTL.
//
// Fail-closed: an inactive token is rejected with ErrInvalidAccessToken; a
// transient store failure propagates as an error rather than being allowed
// through.
func RequireActiveToken(checker ActiveTokenChecker) echo.MiddlewareFunc {
	op := "introspect"

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if isSafeMethod(c.Request().Method) {
				return next(c)
			}

			ctx := c.Request().Context()

			token := xecho.ExtractBearerToken(c.Request())
			if token == "" {
				return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
			}

			user, ok := xecho.UserFromEchoCtx(c)
			if !ok {
				return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
			}

			roles, err := checker.EnsureActiveToken(ctx, token)
			if err != nil {
				return httperrors.ToAPIError(c, op, err)
			}

			current := *user
			current.Roles = roles
			xecho.UserToEchoCtx(c, &current)

			return next(c)
		}
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

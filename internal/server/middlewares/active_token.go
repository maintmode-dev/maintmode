package middlewares

import (
	"context"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// ActiveTokenChecker verifies an access token is still active (not revoked by
// logout, not belonging to a blocked user). It returns nil when active and an
// error otherwise. Declared consumer-side; satisfied in-process by
// *auth.Service.EnsureActiveToken. The middleware only needs the yes/no answer,
// so no claims are returned.
type ActiveTokenChecker interface {
	EnsureActiveToken(ctx context.Context, tokenString string) error
}

// RequireActiveToken verifies the token has not been revoked (logout /
// logout-all) or blocked, by calling the auth service in-process. It must run
// AFTER RequireAccessToken, which has already validated the JWT signature and
// claims locally.
//
// Apply only to critical mutations. Read-only routes intentionally stay on
// local JWT validation.
//
// Fail-closed: an inactive token is rejected with ErrInvalidAccessToken; a
// transient store failure propagates as an error rather than being allowed
// through.
//
// Note on roles: RBAC downstream uses the roles from the JWT claims set by
// RequireAccessToken. Role changes therefore propagate only at token rotation,
// not on every request — this is the documented contract.
func RequireActiveToken(checker ActiveTokenChecker) echo.MiddlewareFunc {
	op := "introspect"

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()

			token := xecho.ExtractBearerToken(c.Request())
			if token == "" {
				return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
			}

			if err := checker.EnsureActiveToken(ctx, token); err != nil {
				return httperrors.ToAPIError(c, op, err)
			}

			return next(c)
		}
	}
}

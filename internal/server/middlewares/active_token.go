package middlewares

import (
	"context"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xhttp"
)

type ActiveTokenIntrospector interface {
	Introspect(ctx context.Context, tokenString string) (*entity.AccessClaims, error)
}

// RequireActiveToken calls auth-gateway introspect to verify the token has not
// been revoked (logout / logout-all). It must run AFTER RequireAccessToken,
// which has already validated the JWT signature and claims locally.
//
// Apply only to critical mutations. Read-only routes intentionally stay on
// local JWT validation so they remain available when auth-gateway is down.
//
// Fail-closed: on network/5xx errors from auth-gateway the request is rejected
// with 503 (via apperr.ErrAuthUnavailable) rather than allowed through.
//
// The introspect deadline is owned by the gateway's HTTP client (configured via
// external_services.auth.timeout). This middleware adds no extra timeout layer
// so that operators have a single tunable knob.
//
// Note on roles: introspect returns fresh roles/email from the auth service,
// but we deliberately discard them. RBAC downstream uses the roles from the
// JWT claims set by RequireAccessToken. Role changes therefore propagate only
// at token rotation, not on every request — this is the documented contract.
func RequireActiveToken(introspector ActiveTokenIntrospector) echo.MiddlewareFunc {
	op := "introspect"

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()

			token := xhttp.ExtractBearerToken(c.Request())
			if token == "" {
				return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
			}

			if _, err := introspector.Introspect(ctx, token); err != nil {
				return httperrors.ToAPIError(c, op, err)
			}

			return next(c)
		}
	}
}

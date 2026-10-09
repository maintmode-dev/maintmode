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

// ActiveTokenChecker verifies an access token is still active. Declared
// consumer-side; satisfied in-process by *auth.Service.
type ActiveTokenChecker interface {
	// EnsureActiveToken is the full check (not revoked, user not blocked) and
	// returns the subject's CURRENT roles from the user store.
	EnsureActiveToken(ctx context.Context, tokenString string) ([]entity.Role, error)
	// EnsureNotRevoked checks revocation only (logout, session revocation):
	// one blacklist lookup, no user-store round trip.
	EnsureNotRevoked(ctx context.Context, tokenString string) error
}

// RequireActiveToken re-checks the token against server-side state on every
// request. It must run AFTER RequireAccessToken (which validated the signature
// and put the user in the context) and BEFORE RequireScenario (which reads the
// roles it may replace).
//
// Every method is refused once the token is revoked -- on its own (logout) or
// through its session (logout-all, reuse detection, a password change, a
// block). Without that a cut-off session could keep reading until its token
// expires. Costs one Valkey EXISTS per request.
//
// Writes additionally get the blocked-user check and RBAC on the user's stored
// roles rather than the ones baked into the JWT; without it a blocked or demoted
// user keeps every write their token's claims allow until the token expires --
// enough to unblock themselves or re-grant a revoked role. Safe methods
// (GET/HEAD/OPTIONS) skip that part: it is a user-store round trip, reads are
// the hot path, and a block already revokes the user's sessions.
//
// Fail-closed: an inactive token is rejected with ErrInvalidAccessToken; a
// transient store failure propagates as an error rather than being allowed
// through.
func RequireActiveToken(checker ActiveTokenChecker) echo.MiddlewareFunc {
	op := "introspect"

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()

			token := xecho.ExtractBearerToken(c.Request())
			if token == "" {
				return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
			}

			user, ok := xecho.UserFromEchoCtx(c)
			if !ok {
				return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
			}

			if isSafeMethod(c.Request().Method) {
				if err := checker.EnsureNotRevoked(ctx, token); err != nil {
					return httperrors.ToAPIError(c, op, err)
				}
				return next(c)
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

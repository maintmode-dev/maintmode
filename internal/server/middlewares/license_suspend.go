package middlewares

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// LicenseProvider yields the current license snapshot. nil means no license is
// known (fresh SaaS instance before its first heartbeat) — nothing to enforce.
// Implemented by license.Service.
type LicenseProvider interface {
	License(context.Context) *entity.License
}

// RequireLicenseNotSuspended is the license block gate: while the cached
// license is blocked, every MUTATING request is rejected with 403
// organization_suspended (the FE turns that code into the full-screen suspended
// page). Read requests (GET/HEAD/OPTIONS) still pass, so a blocked organization
// stays fully browsable — its members can read maintenances, events and
// resources, they just cannot change anything. Data is preserved; the next
// heartbeat lifts the gate ≤1 min after Console unblocks.
//
// Wired on the business route groups only (not the auth module), so the whole
// auth surface — login, token refresh, invitation accept, logout, /me and the
// role/user admin — stays reachable under a blocked license: a member of a
// blocked org must still be able to sign in, manage their session and see the
// suspended page.
func RequireLicenseNotSuspended(provider LicenseProvider) echo.MiddlewareFunc {
	op := "license block gate"

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if isMutating(c.Request().Method) && provider.License(context.Background()).Blocked() {
				return httperrors.ToAPIError(c, op, apperr.ErrOrganizationSuspended)
			}
			return next(c)
		}
	}
}

// isMutating reports whether the HTTP method changes server state. The safe,
// read-only methods stay available under a blocked license.
func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

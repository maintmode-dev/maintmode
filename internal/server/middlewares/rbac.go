package middlewares

import (
	"context"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

type Authorizer interface {
	Allow(ctx context.Context, roles []entity.Role, scenario entity.AuthzScenario) (bool, error)
}

func RequireScenario(authorizer Authorizer, scenario entity.AuthzScenario) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()
			op := "authorize"

			user, ok := xecho.UserFromEchoCtx(c)
			if !ok {
				return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
			}

			allowed, err := authorizer.Allow(ctx, user.Roles, scenario)
			if err != nil {
				xlog.Error(ctx, "authorize failed",
					xfield.String("scenario", string(scenario)),
					xfield.Error(err),
				)
				return httperrors.ToAPIError(c, op, err)
			}
			if !allowed {
				return httperrors.ToAPIError(c, op, apperr.ErrForbidden)
			}

			return next(c)
		}
	}
}

// RequireScenarioUnlessSelf lets a caller act on their OWN record, named by the
// path parameter param, and requires scenario for anyone else's. It is for
// reads that are harmless about yourself and a reconnaissance tool about
// others: any guest reading every user's roles has a ready list of who the
// admins are, while every user may read their own.
//
// A malformed id is not the caller's own, so it falls to the scenario check
// rather than slipping through.
func RequireScenarioUnlessSelf(authorizer Authorizer, scenario entity.AuthzScenario, param string) echo.MiddlewareFunc {
	requireScenario := RequireScenario(authorizer, scenario)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		guarded := requireScenario(next)

		return func(c *echo.Context) error {
			user, ok := xecho.UserFromEchoCtx(c)
			if ok && c.Param(param) == user.ID.String() {
				return next(c)
			}

			return guarded(c)
		}
	}
}

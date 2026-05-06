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

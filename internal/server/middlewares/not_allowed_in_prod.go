package middlewares

import (
	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
)

func NotAllowedInProd(env config.Environment) echo.MiddlewareFunc {
	op := "allowed in dev only "
	isProd := env.IsProd()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if isProd {
				return httperrors.ToAPIError(c, op, apperr.ErrMethodNotAllowedInProd)
			}
			return next(c)
		}
	}
}

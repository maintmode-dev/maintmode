package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// LoginWithBreakGlass godoc
// @Summary Sign in with the break-glass password
// @Description Issues a backend token pair for the break-glass admin account. The body carries the configured break-glass password and nothing else -- there is no address. Every failure (wrong password, break-glass switched off, blocked account, malformed body) answers with the same 401.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.LoginWithBreakGlassRequest true "Break-glass password"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 401 {object} httperrors.ErrorResponse "Authentication failed"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/login/break-glass [post]
func (i *Implementation) LoginWithBreakGlass(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.LoginWithBreakGlass")
	defer span.End()

	body := new(apiauthmodels.LoginWithBreakGlassRequest)
	if err := c.Bind(body); err != nil {
		return unauthorized(ctx, c, "malformed request body", err)
	}

	pair, err := i.authSrv.LoginWithBreakGlass(ctx, &entity.LoginWithBreakGlassCmd{
		Password:  body.Password,
		ClientIP:  c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	})
	if err != nil {
		return unauthorized(ctx, c, "authentication failed", err)
	}

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

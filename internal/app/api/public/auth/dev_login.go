package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// DevLogin godoc
// @Summary Sign in as a fresh user with the given roles (dev only)
// @Description DEV ONLY: registered only in a dev environment that runs the stub login provider, and absent in prod, where it answers 404. Creates a new user on every call -- invite-only signup notwithstanding -- grants the requested roles on top of the defaults and issues a token pair. It takes no provider credential: it is how dev tooling and the API tests get a session with given roles.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.DevLoginRequest true "Roles to grant"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 400 {object} httperrors.ErrorResponse "Malformed body, unknown role, or no google login provider on the stand"
// @Failure 401 {object} httperrors.ErrorResponse "invalid_client: the caller is not an authenticated service"
// @Failure 403 {object} httperrors.ErrorResponse "Seats limit exceeded"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security S2SAppName || S2SToken
// @Router /api/v1/login/dev [post]
func (i *Implementation) DevLogin(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.DevLogin")
	defer span.End()
	op := "auth dev login"

	body := new(apiauthmodels.DevLoginRequest)
	if err := c.Bind(body); err != nil {
		xlog.Error(ctx, "failed to bind dev login request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	pair, err := i.authSrv.DevSignIn(ctx, &entity.DevSignInCmd{
		Roles:     lo.Map(body.Roles, func(role string, _ int) entity.Role { return entity.Role(role) }),
		ClientIP:  c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	})
	if err != nil {
		xlog.Error(ctx, "dev sign-in failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

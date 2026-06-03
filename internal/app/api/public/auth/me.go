package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// Me godoc
// @Summary Get current authenticated user
// @Description Returns the profile of the user identified by the Bearer access token.
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} apiauthmodels.MeResponse
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/me [get]
// Me returns the current user's profile.
func (i *Implementation) Me(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.Me")
	defer span.End()
	op := "get current user"

	ctxUser, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		xlog.Error(ctx, "missing user in context")
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	user, err := i.userSrv.GetByID(ctx, ctxUser.ID)
	if err != nil {
		xlog.Error(ctx, "failed to get user", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	providers, err := i.userSrv.ListConnectedProviders(ctx, ctxUser.ID)
	if err != nil {
		xlog.Error(ctx, "failed to list connected providers", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPIMeResponse(user, providers))
}

package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// DisconnectProvider godoc
// @Summary Disconnect an OAuth provider from the current user
// @Description Unlinks a sign-in provider from the authenticated user. The last
// @Description remaining provider cannot be removed (lockout protection).
// @Description Disconnecting a provider the user is not linked to is a no-op (204).
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Param provider path string true "Configured provider instance name, e.g. google"
// @Success 204 "Provider disconnected"
// @Failure 400 {object} httperrors.ErrorResponse "Cannot disconnect the only sign-in method"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/me/providers/{provider}/disconnect [delete]
func (i *Implementation) DisconnectProvider(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.DisconnectProvider")
	defer span.End()
	op := "disconnect provider"

	ctxUser, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		xlog.Error(ctx, "missing user in context")
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	err := i.authSrv.DisconnectProvider(ctx, &entity.DisconnectProviderCmd{
		UserID:   ctxUser.ID,
		Provider: c.Param("provider"),
	})
	if err != nil {
		xlog.Error(ctx, "disconnect provider failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

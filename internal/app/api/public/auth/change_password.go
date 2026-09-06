package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// ChangePassword godoc
// @Summary Set or change your own password
// @Description Writes the caller's password, retires any pending break-glass seed, and revokes their other sessions. Supplying refresh_token keeps that session alive; omitting it revokes every session including the caller's.
// @Tags Auth
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer access token"
// @Param request body apiauthmodels.ChangePasswordRequest true "Password change"
// @Success 204 "Password changed"
// @Failure 400 {object} httperrors.ErrorResponse "Validation failed"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/me/password [post]
//
// No token pair is returned. The caller either kept their session (and its
// tokens still work) or asked for every session to go, in which case handing
// back a fresh one would defeat the request.
func (i *Implementation) ChangePassword(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.ChangePassword")
	defer span.End()
	op := "change-password"

	user, ok := xecho.UserFromEchoCtx(c)
	if !ok || user == nil {
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	body := new(apiauthmodels.ChangePasswordRequest)
	if err := c.Bind(body); err != nil {
		xlog.Warn(ctx, "invalid change password request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(apperr.ErrValidation))
	}

	err := i.authSrv.ChangePassword(ctx, &entity.ChangePasswordCmd{
		UserID:          user.ID,
		CurrentPassword: body.CurrentPassword,
		NewPassword:     body.NewPassword,
		RefreshToken:    body.RefreshToken,
		ClientIP:        c.RealIP(),
		UserAgent:       c.Request().UserAgent(),
	})
	if err != nil {
		xlog.Warn(ctx, "password change rejected", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

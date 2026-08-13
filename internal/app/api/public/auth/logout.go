package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xecho"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Logout godoc
// @Summary Logout current session
// @Description Revokes the current refresh token and blacklists the access token.
// @Tags Auth
// @Accept json
// @Produce json
// @Param Authorization header string false "Bearer access token"
// @Param request body refreshTokenJSONRequest true "Refresh token"
// @Success 204 "Logged out"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid token"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/logout [post]
// Logout revokes the current refresh token and blacklists the access token. The
// caller sends the refresh token in the request body and the access token as a
// Bearer header.
func (i *Implementation) Logout(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.Logout")
	defer span.End()
	op := "logout"

	refreshToken, err := extractRefreshToken(c)
	if err != nil {
		xlog.Error(ctx, "missing refresh token", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(apperr.ErrInvalidRefreshToken))
	}

	err = i.authSrv.Logout(ctx, &entity.TokenPair{
		RefreshToken: refreshToken,
		AccessToken:  xecho.ExtractBearerToken(c.Request()),
	})
	if err != nil {
		xlog.Error(ctx, "invalid token", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// LogoutAll godoc
// @Summary Logout from all sessions
// @Description Revokes all refresh tokens for the current user.
// @Tags Auth
// @Produce json
// @Param Authorization header string true "Bearer access token"
// @Success 204 "Logged out from all sessions"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid token"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/logout/all [post]
// LogoutAll revokes all refresh tokens for the authenticated user.
func (i *Implementation) LogoutAll(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.LogoutAll")
	defer span.End()
	op := "logout all"

	err := i.authSrv.LogoutAll(ctx, xecho.ExtractBearerToken(c.Request()))
	if err != nil {
		xlog.Error(ctx, "invalid access token", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

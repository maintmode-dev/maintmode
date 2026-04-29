package auth

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Logout godoc
// @Summary Logout current session
// @Description Revokes current refresh token, blacklists access token, and clears refresh cookie.
// @Tags Auth
// @Accept json
// @Produce json
// @Param Authorization header string false "Bearer access token"
// @Param request body refreshTokenJSONRequest false "Fallback refresh token body when cookie is absent"
// @Success 204 "Logged out"
// @Failure 400 {object} apierrors.ErrorResponse "Invalid token"
// @Failure 401 {object} apierrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Header 204 {string} Set-Cookie "Cleared refresh token cookie"
// @Router /api/v1/logout [post]
// Logout revokes the current refresh token and blacklists the access token.
//
// If refresh token is absent (e.g. long-lived tab where cookie was already cleared),
// we just clear the cookie and return success — nothing to revoke on the server.
func (i *Implementation) Logout(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.Logout")
	defer span.End()
	op := "logout"

	refreshToken, err := extractRefreshToken(ctx, c)
	if err != nil {
		xlog.Error(ctx, "missing refresh token", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apperr.ErrInvalidRefreshTokenToken)
		return c.JSON(statusCode, errResp)
	}

	// No refresh token — just clear the cookie.
	if refreshToken == "" {
		clearRefreshCookie(c)
		return c.NoContent(http.StatusNoContent)
	}

	err = i.authSrv.Logout(ctx, &entity.TokenPair{
		RefreshToken: refreshToken,
		AccessToken:  extractBearerToken(c),
	})
	if err != nil {
		xlog.Error(ctx, "invalid token", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	clearRefreshCookie(c)
	return c.NoContent(http.StatusNoContent)
}

// LogoutAll godoc
// @Summary Logout from all sessions
// @Description Revokes all refresh tokens for current user and clears refresh cookie.
// @Tags Auth
// @Produce json
// @Param Authorization header string true "Bearer access token"
// @Success 204 "Logged out from all sessions"
// @Failure 400 {object} apierrors.ErrorResponse "Invalid token"
// @Failure 401 {object} apierrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Header 204 {string} Set-Cookie "Cleared refresh token cookie"
// @Router /api/v1/logout/all [post]
// LogoutAll revokes all refresh tokens for the authenticated user.
func (i *Implementation) LogoutAll(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.LogoutAll")
	defer span.End()
	op := "logout all"

	err := i.authSrv.LogoutAll(ctx, extractBearerToken(c))
	if err != nil {
		xlog.Error(ctx, "invalid access token", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	clearRefreshCookie(c)
	return c.NoContent(http.StatusNoContent)
}

const bearerHeader = "Bearer "

// extractBearerToken returns the raw Bearer token from the Authorization header,
// or empty string if absent/malformed.
func extractBearerToken(c *echo.Context) string {
	authHeader := c.Request().Header.Get("Authorization")

	token, ok := strings.CutPrefix(authHeader, bearerHeader)
	if !ok {
		return ""
	}

	return token
}

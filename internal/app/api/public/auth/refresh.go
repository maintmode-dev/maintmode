package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
)

// Refresh godoc
// @Summary Refresh token pair
// @Description Rotates refresh token and issues a new access token pair.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body refreshTokenJSONRequest false "Fallback refresh token body when cookie is absent"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 400 {object} apierrors.ErrorResponse "Invalid refresh token"
// @Failure 409 {object} apierrors.ErrorResponse "Refresh lock busy or token reuse"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Header 200 {string} Set-Cookie "Rotated refresh token cookie (if issued)"
// @Router /api/v1/refresh [post]
// Refresh rotates the refresh token and issues a new token pair.
func (i *Implementation) Refresh(c echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.Refresh")
	defer span.End()
	op := "rotate refresh token"

	refreshToken, err := extractRefreshToken(ctx, c)
	if err != nil {
		xlog.Error(ctx, "missing refresh token", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apperr.ErrInvalidRefreshTokenToken)
		return c.JSON(statusCode, errResp)
	}

	pair, err := i.authSrv.Refresh(ctx, refreshToken, c.RealIP())
	if err != nil {
		if errors.Is(err, apperr.ErrLockBusy) {
			c.Response().Header().Set("Retry-After", "1")
		}

		xlog.Error(ctx, "refresh token failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	if pair.RefreshToken != "" {
		setRefreshCookie(c, pair.RefreshToken)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

type refreshTokenJSONRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func extractRefreshToken(ctx context.Context, c echo.Context) (string, error) {
	cookie, err := c.Cookie(cookieRefreshTokenName)
	if err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}
	xlog.Warn(ctx, "missing refresh token in cookie", xfield.String("cookie name", cookieRefreshTokenName))

	// Fallback: JSON body
	body := new(refreshTokenJSONRequest)
	if err := c.Bind(body); err != nil {
		return "", fmt.Errorf("extract refresh token from json: %w", err)
	}
	return body.RefreshToken, nil
}

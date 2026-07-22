package auth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
)

// Refresh godoc
// @Summary Refresh token pair
// @Description Rotates refresh token and issues a new access token pair.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body refreshTokenJSONRequest true "Refresh token"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid refresh token"
// @Failure 409 {object} httperrors.ErrorResponse "Refresh lock busy or token reuse"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/refresh [post]
// Refresh rotates the refresh token and issues a new token pair. The caller
// (the BFF) sends the current refresh token in the request body and stores the
// rotated one from the response body; no cookie is involved.
func (i *Implementation) Refresh(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.Refresh")
	defer span.End()
	op := "rotate refresh token"

	refreshToken, err := extractRefreshToken(c)
	if err != nil {
		xlog.Error(ctx, "missing refresh token", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(apperr.ErrInvalidRefreshToken))
	}

	pair, err := i.authSrv.Refresh(ctx, refreshToken, c.RealIP())
	if err != nil {
		if errors.Is(err, apperr.ErrLockBusy) {
			c.Response().Header().Set("Retry-After", "1")
		}

		xlog.Error(ctx, "refresh token failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

type refreshTokenJSONRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// extractRefreshToken reads the refresh token from the JSON request body. The
// BFF owns token storage and always sends the token in the body; the legacy
// refresh cookie is gone.
func extractRefreshToken(c *echo.Context) (string, error) {
	body := new(refreshTokenJSONRequest)
	if err := c.Bind(body); err != nil {
		return "", fmt.Errorf("extract refresh token from json: %w", err)
	}
	return body.RefreshToken, nil
}

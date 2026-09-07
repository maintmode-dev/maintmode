package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
)

// ExchangeOAuthDanceCode godoc
// @Summary Redeem the one-time code from an OAuth dance
// @Description Trades the short-lived opaque code the callback put in the redirect for the token pair it stands for. Single-use: the second attempt with the same code fails like any other. Every failure — unknown, expired, already redeemed, malformed — answers the same 401, so a caller cannot learn which of its guesses was closer.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.ExchangeOAuthCodeRequest true "The one-time code"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 401 {object} httperrors.ErrorResponse "The code is not redeemable"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Router /api/v1/login/oauth/code/exchange [post]
func (i *Implementation) ExchangeOAuthDanceCode(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.OAuthDance.ExchangeCode")
	defer span.End()
	op := "oauth dance code exchange"

	body := new(apiauthmodels.ExchangeOAuthCodeRequest)
	if err := c.Bind(body); err != nil {
		// A malformed body answers exactly as a wrong code does. Distinguishing
		// them would tell a prober that its JSON was at least well-formed, which
		// is the first bit of a guess.
		xlog.Warn(ctx, "failed to bind oauth code exchange request", xfield.Error(err))

		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	if body.Code == "" {
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	pair, err := i.authSrv.RedeemDanceCode(ctx, body.Code)
	if err != nil {
		xlog.Error(ctx, "failed to consume the one-time dance code", xfield.Error(err))

		// Even a store fault answers 401 rather than 500: the uniform response
		// is the property, and a 500 here would mark the one code whose lookup
		// misbehaved.
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	// Unknown, expired and already-redeemed are one case by design. The audit
	// trail is where they stay tellable apart.
	if pair == nil {
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")

	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

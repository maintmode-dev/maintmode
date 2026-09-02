package auth

import (
	"context"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// ExchangeGoogleToken godoc
// @Summary Exchange a Google ID token for a backend token pair
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.ExchangeIDTokenRequest true "Google ID token"
// @Success 200 {object} apiauthmodels.TokenPairResponse
// @Failure 400 {object} httperrors.ErrorResponse "INVALID_ID_TOKEN or EMAIL_NOT_VERIFIED"
// @Failure 403 {object} httperrors.ErrorResponse "DOMAIN_NOT_ALLOWED or signup_disabled"
// @Failure 429 {object} httperrors.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/login/oauth/exchange/google [post]
// Exchange verifies a Google ID token and issues a backend token pair.
func (i *Implementation) ExchangeGoogleToken(c *echo.Context) error {
	return i.exchange(c, entity.AuthMethodGoogle)
}

func (i *Implementation) exchange(c *echo.Context, provider entity.AuthMethod) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), fmt.Sprintf("api.Auth.Exchange.%s", provider))
	defer span.End()
	op := "auth exchange"

	body := new(apiauthmodels.ExchangeIDTokenRequest)
	if err := c.Bind(body); err != nil {
		xlog.Error(ctx, "failed to bind exchange request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	// Dev-only test roles, if the X-Test-Roles middleware ran (it is registered
	// only under the dev environment gate). Absent in production → nil.
	testRoles, _ := xecho.TestRolesFromEchoCtx(c)

	cmd := &entity.ExchangeIDTokenCmd{
		Provider:  provider,
		IDToken:   body.IDToken,
		ClientIP:  c.RealIP(),
		UserAgent: c.Request().UserAgent(),
		TestRoles: testRoles,
	}

	if err := validateExchangeIDTokenCmd(ctx, cmd); err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	pair, err := i.authSrv.ExchangeIDToken(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "exchange id token failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	return c.JSON(http.StatusOK, apiauthmodels.ToAPITokenPairResponse(pair))
}

func validateExchangeIDTokenCmd(ctx context.Context, cmd *entity.ExchangeIDTokenCmd) error {
	return validation.ValidateStructWithContext(ctx, cmd,
		validation.Field(&cmd.Provider, validation.Required),
		validation.Field(&cmd.IDToken, validation.Required),
		validation.Field(&cmd.ClientIP, validation.Required),
	)
}

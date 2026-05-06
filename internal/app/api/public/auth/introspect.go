package auth

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
)

// Introspect godoc
// @Summary Introspect access token
// @Description Validates access token and returns its activity and claims (S2S endpoint).
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.IntrospectRequest true "Introspect request"
// @Success 200 {object} apiauthmodels.IntrospectResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Invalid service token"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Router /api/v1/s2s/introspect [post]
// Introspect implements AccessToken Introspection (RFC 7662).
// Downstream services call this for critical operations to check if a token is blacklisted.
func (i *Implementation) Introspect(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.Introspect")
	defer span.End()
	op := "introspect"

	body := new(apiauthmodels.IntrospectRequest)
	if err := c.Bind(body); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if body.AccessToken == "" {
		err := fmt.Errorf("missing token")
		xlog.Error(ctx, "missing token", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	report, err := i.authSrv.Introspect(ctx, body.AccessToken)
	if err != nil {
		xlog.Error(ctx, "introspect failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPIIntrospectResponse(report))
}

package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
	"github.com/ruko1202/maintmode/internal/apperr"
)

// Introspect godoc
// @Summary Introspect access token
// @Description Validates access token and returns its activity and claims (S2S endpoint).
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body apiauthmodels.IntrospectRequest true "Introspect request"
// @Success 200 {object} apiauthmodels.IntrospectResponse
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request"
// @Failure 401 {object} apierrors.ErrorResponse "Invalid service token"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/s2s/introspect [post]
// Introspect implements AccessToken Introspection (RFC 7662).
// Downstream services call this for critical operations to check if a token is blacklisted.
func (i *Implementation) Introspect(c echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.Introspect")
	defer span.End()
	op := "introspect"

	body := new(apiauthmodels.IntrospectRequest)
	err := c.Bind(body)
	if err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrParseBody)
		return c.JSON(statusCode, errResp)
	}

	if body.AccessToken == "" {
		xlog.Error(ctx, "missing token", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apperr.ErrInvalidAccessTokenToken)
		return c.JSON(statusCode, errResp)
	}

	report, err := i.authSrv.Introspect(ctx, body.AccessToken)
	if err != nil {
		xlog.Error(ctx, "introspect failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPIIntrospectResponse(report))
}

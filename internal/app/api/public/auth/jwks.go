package auth

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"

	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/auth/models"
)

// JWKS godoc
// @Summary Get JWKS
// @Description Returns JSON Web Key Set used to verify access tokens.
// @Tags Auth
// @Produce json
// @Success 200 {object} apiauthmodels.JWKSResponse
// @Router /api/v1/.well-known/jwks.json [get]
// JWKS serves the JSON Web Key Set for token verification by downstream services.
func (i *Implementation) JWKS(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Auth.JWKS")
	defer span.End()

	jwks := i.tokenSrv.JWKS(ctx)

	return c.JSON(http.StatusOK, apiauthmodels.ToAPIJWKSResponse(jwks))
}

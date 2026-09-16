package integrationapi

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
)

// Get godoc
// @Summary Get an integration by kind
// @Description Returns the masked integration for a kind. Admin only.
// @Tags Integrations
// @Produce json
// @Param kind path string true "Category: notify or login"
// @Param name path string true "System within the category: slack, telegram, email, google or custom"
// @Success 200 {object} apimodels.Integration
// @Failure 403 {object} httperrors.ErrorResponse
// @Failure 404 {object} httperrors.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/integrations/{kind}/{name} [get]
func (i *Implementation) Get(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Integration.Get")
	defer span.End()
	op := "get integration"

	masked, err := i.integrationSrv.GetByKindName(ctx, c.Param("kind"), c.Param("name"))
	if err != nil {
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, i.toAPIWithAuthorship(ctx, masked))
}

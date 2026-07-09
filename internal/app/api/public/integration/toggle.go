package integrationapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/integration/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// Toggle godoc
// @Summary Enable/disable an integration
// @Description Flips the enabled flag at runtime without resending settings. Admin only.
// @Tags Integrations
// @Accept json
// @Produce json
// @Param kind path string true "Integration kind"
// @Param request body apimodels.ToggleIntegrationRequest true "Toggle request"
// @Success 200 {object} apimodels.Integration
// @Failure 400 {object} httperrors.ErrorResponse
// @Failure 403 {object} httperrors.ErrorResponse
// @Failure 404 {object} httperrors.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/integrations/{kind}/toggle [post]
func (i *Implementation) Toggle(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Integration.Toggle")
	defer span.End()
	op := "toggle integration"

	req := new(apimodels.ToggleIntegrationRequest)
	if err := c.Bind(req); err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(fmt.Errorf("actor not found")))
	}

	masked, err := i.integrationSrv.Toggle(ctx, &entity.ToggleIntegrationCmd{
		Kind:    c.Param("kind"),
		Enabled: req.Enabled,
		Actor:   actor,
	})
	if err != nil {
		xlog.Error(ctx, "toggle integration failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, i.toAPIWithAuthorship(ctx, masked))
}

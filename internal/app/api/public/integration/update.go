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

// Update godoc
// @Summary Update an integration
// @Description Updates an integration's config and (partially) secrets. Admin only. A secret key omitted keeps its stored value.
// @Tags Integrations
// @Accept json
// @Produce json
// @Param kind path string true "Integration kind"
// @Param request body apimodels.UpdateIntegrationRequest true "Update integration request"
// @Success 200 {object} apimodels.Integration
// @Failure 400 {object} httperrors.ErrorResponse
// @Failure 403 {object} httperrors.ErrorResponse
// @Failure 404 {object} httperrors.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/integrations/{kind} [patch]
func (i *Implementation) Update(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Integration.Update")
	defer span.End()
	op := "update integration"

	req := new(apimodels.UpdateIntegrationRequest)
	if err := c.Bind(req); err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(fmt.Errorf("actor not found")))
	}

	masked, err := i.integrationSrv.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    c.Param("kind"),
		Enabled: req.Enabled,
		Config:  req.Config,
		Secrets: req.Secrets,
		Actor:   actor,
	})
	if err != nil {
		xlog.Error(ctx, "update integration failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, i.toAPIWithAuthorship(ctx, masked))
}

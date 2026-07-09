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

// Create godoc
// @Summary Create an integration
// @Description Creates an integration of a kind. Admin only. Secrets are encrypted at rest; the response masks them.
// @Tags Integrations
// @Accept json
// @Produce json
// @Param request body apimodels.CreateIntegrationRequest true "Create integration request"
// @Success 200 {object} apimodels.Integration
// @Failure 400 {object} httperrors.ErrorResponse
// @Failure 403 {object} httperrors.ErrorResponse
// @Failure 409 {object} httperrors.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/integrations [post]
func (i *Implementation) Create(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Integration.Create")
	defer span.End()
	op := "create integration"

	req := new(apimodels.CreateIntegrationRequest)
	if err := c.Bind(req); err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(fmt.Errorf("actor not found")))
	}

	masked, err := i.integrationSrv.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    req.Kind,
		Enabled: req.Enabled,
		Config:  req.Config,
		Secrets: req.Secrets,
		Actor:   actor,
	})
	if err != nil {
		xlog.Error(ctx, "create integration failed", xfield.String("kind", req.Kind), xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, i.toAPIWithAuthorship(ctx, masked))
}

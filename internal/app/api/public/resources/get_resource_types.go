package resourcesapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
)

// GetResourceTypes godoc
// @Summary Get available resource types
// @Description Returns available resource types for a specific resource
// @Tags Resources
// @Produce json
// @Param id path string true "Resource ID" format(uuid)
// @Success 200 {object} apimodels.GetResourceTypesResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Resource not found"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/resource/{id}/types [get]
func (i *Implementation) GetResourceTypes(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.GetResourceTypes")
	defer span.End()
	op := "get resource types"

	resourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse resourceID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	types, err := i.resourcesSrv.GetResourceTypes(ctx, resourceID)
	if err != nil {
		xlog.Error(ctx, "get resource types failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, &apimodels.GetResourceTypesResponse{
		Types: apimodels.ToAPIResourceTypes(types),
	})
}

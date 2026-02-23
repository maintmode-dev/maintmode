package resourcesapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
)

// GetResourceTypes godoc
// @Summary Get available resource types
// @Description Returns available resource types for a specific resource
// @Tags Resources
// @Produce json
// @Param id path string true "Resource ID" format(uuid)
// @Success 200 {object} apimodels.GetResourceTypesResponse
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request"
// @Failure 404 {object} apierrors.ErrorResponse "Resource not found"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/resource/{id}/types [get]
func (i *Implementation) GetResourceTypes(c echo.Context) error {
	ctx := xlog.WithOperation(c.Request().Context(), "api.Resources.GetResourceTypes")
	op := "get resource types"

	resourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse uuid failed", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	types, err := i.resourcesSrv.GetResourceTypes(ctx, resourceID)
	if err != nil {
		xlog.Error(ctx, "get resource types failed", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	return c.JSON(http.StatusOK, &apimodels.GetResourceTypesResponse{
		Types: apimodels.ToAPIResourceTypes(types),
	})
}

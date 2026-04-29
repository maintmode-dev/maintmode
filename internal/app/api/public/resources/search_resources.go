package resourcesapi

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
)

// SearchResources godoc
// @Summary Search resources by name
// @Description Searches for resources by name using LIKE pattern matching
// @Tags Resources
// @Produce json
// @Param name query string true "Resource name to search for"
// @Success 200 {object} apismodels.SearchResourcesResponse
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/resources [get]
func (i *Implementation) SearchResources(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.SearchResources")
	defer span.End()
	op := "search resources"

	req := new(apimodels.SearchResourcesRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	resources, err := i.resourcesSrv.GetResourcesLikeName(ctx, req.Name)
	if err != nil {
		xlog.Error(ctx, "search resources failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	return c.JSON(http.StatusOK, &apimodels.SearchResourcesResponse{
		Items: apimodels.ToAPIResources(resources),
	})
}

package resourcesapi

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
)

// SearchResources godoc
// @Summary Search resources by name
// @Description Searches for resources by name using LIKE pattern matching
// @Tags Resources
// @Produce json
// @Param name query string true "Resource name to search for"
// @Success 200 {object} apimodels.SearchResourcesResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/resources [get]
func (i *Implementation) SearchResources(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.SearchResources")
	defer span.End()
	op := "search resources"

	req := new(apimodels.SearchResourcesRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	resources, err := i.resourcesSrv.GetResourcesLikeName(ctx, req.Name)
	if err != nil {
		xlog.Error(ctx, "search resources failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// Batch-resolve every author/editor id across the page in one auth call
	// (degrades to "Unknown user" on failure, never errors the read).
	summaries := i.userSummarySrv.ResolveMany(ctx, apimodels.ResourceUserIDs(resources))

	return c.JSON(http.StatusOK, &apimodels.SearchResourcesResponse{
		Items: apimodels.ToAPIResources(resources, summaries),
	})
}

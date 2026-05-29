package resourcesapi

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/resources/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	defaultResourcesLimit  = 50
	maxResourcesLimit      = 200
	defaultResourcesOffset = 0
)

// ListResources godoc
// @Summary List resources
// @Description Returns a paginated list of resources ordered by created_at DESC (newest first).
// @Description By default only active resources are returned; pass archived=true to include archived ones as well.
// @Description Malformed pagination/filter params are coerced to defaults rather than rejected.
// @Tags Resources
// @Produce json
// @Param name query string false "Case-insensitive partial name match (search box)"
// @Param limit query int false "Page size (max 200)" default(50)
// @Param offset query int false "Pagination offset" default(0)
// @Param archived query bool false "Include archived resources (false: active only; true: active + archived)" default(false)
// @Success 200 {object} apimodels.ListResourcesResponse
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/resources/list [get]
func (i *Implementation) ListResources(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.ListResources")
	defer span.End()
	op := "list resources"

	cmd := queryToListResourcesCmd(c)

	result, err := i.resourcesSrv.ListResources(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "list resources failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, &apimodels.ListResourcesResponse{
		Resources: apimodels.ToAPIResources(result.Resources),
		Total:     result.Total,
		Limit:     cmd.Limit,
		Offset:    cmd.Offset,
	})
}

// queryToListResourcesCmd parses pagination/filter query params. Malformed or
// out-of-range values are silently coerced to safe defaults rather than
// rejected — this is a read-only reference list, so a best-effort response is
// preferable to a 400.
func queryToListResourcesCmd(c *echo.Context) *entity.ListResourcesCmd {
	limit, err := echo.QueryParamOr[int64](c, "limit", defaultResourcesLimit)
	if err != nil || limit <= 0 || limit > maxResourcesLimit {
		limit = defaultResourcesLimit
	}

	offset, err := echo.QueryParamOr[int64](c, "offset", defaultResourcesOffset)
	if err != nil || offset < 0 {
		offset = defaultResourcesOffset
	}

	includeArchived, err := echo.QueryParamOr[bool](c, "archived", false)
	if err != nil {
		includeArchived = false
	}

	return &entity.ListResourcesCmd{
		Name:            c.QueryParam("name"),
		Limit:           limit,
		Offset:          offset,
		IncludeArchived: includeArchived,
	}
}

package resourcesapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
)

// ArchiveResource godoc
// @Summary Archive a resource
// @Description Marks a resource as archived. Idempotent: archiving an already-archived or unknown resource succeeds.
// @Tags Resources
// @Produce json
// @Param id path string true "Resource ID" Format(uuid)
// @Success 204 "Resource archived"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid resource id"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/resource/{id}/archive [post]
func (i *Implementation) ArchiveResource(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Resources.ArchiveResource")
	defer span.End()
	op := "archive resource"

	resourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse resourceID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	if err := i.resourcesSrv.ArchiveResource(ctx, resourceID); err != nil {
		xlog.Error(ctx, "archive resource failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

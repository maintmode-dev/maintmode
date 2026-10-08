package resourcesapi

import (
	"github.com/labstack/echo/v5"
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
	return changeArchiveState(c, "api.Resources.ArchiveResource", "archive resource", i.resourcesSrv.ArchiveResource)
}

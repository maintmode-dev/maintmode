package resourcesapi

import (
	"github.com/labstack/echo/v5"
)

// UnarchiveResource godoc
// @Summary Unarchive a resource
// @Description Restores a resource to active. Idempotent: unarchiving an already-active or unknown resource succeeds.
// @Tags Resources
// @Produce json
// @Param id path string true "Resource ID" Format(uuid)
// @Success 204 "Resource unarchived"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid resource id"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/resource/{id}/unarchive [post]
func (i *Implementation) UnarchiveResource(c *echo.Context) error {
	return changeArchiveState(c, "api.Resources.UnarchiveResource", "unarchive resource", i.resourcesSrv.UnarchiveResource)
}

//nolint:dupl // to skip 1-66 lines are duplicate of `internal/app/api/maint/start_maint.go:1-66`
package apimaint

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/entity"
)

// CompleteMaint godoc
// @Summary Complete maintenance
// @Description Complete a maintenance by ID. Allowed only for valid status transitions.
// @Tags Maintenances
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Success 204 "Maintenance completed"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid UUID"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 409 {object} httperrors.ErrorResponse "Forbidden status transition or maintenance has unfinished steps"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/maintenances/{id}/complete [post]
func (i *Implementation) CompleteMaint(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.CompleteMaint")
	defer span.End()
	op := "complete maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	err = i.maintSrv.CompleteMaint(ctx, &entity.CompleteMaintenanceCmd{MaintID: maintID})
	if err != nil {
		xlog.Error(ctx, "complete maintenance failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

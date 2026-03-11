//nolint:dupl // to skip 1-66 lines are duplicate of `internal/app/api/maint/start_maint.go:1-66`
package apimaint

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	"github.com/ruko1202/maintmode/internal/entity"
)

// CompleteMaint godoc
// @Summary Complete maintenance
// @Description Complete a maintenance by ID. Allowed only for valid status transitions.
// @Tags Maintenances
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Success 204 "Maintenance completed"
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request or forbidden status transition"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/maintenances/{id}/complete [post]
func (i *Implementation) CompleteMaint(c echo.Context) error {
	ctx := xlog.WithOperation(c.Request().Context(), "api.Maint.CompleteMaint")
	op := "complete maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse uuid failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	err = i.maintSrv.Complete(ctx, &entity.CompleteMaintenanceCmd{MaintID: maintID})
	if err != nil {
		xlog.Error(ctx, "complete maintenance failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	return c.NoContent(http.StatusNoContent)
}

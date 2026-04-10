//nolint:dupl  //to skip 1-66 lines are duplicate of `internal/app/api/maint/cancel_maint.go:2-65`
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

// StartMaint godoc
// @Summary Start maintenance
// @Description Starts a maintenance by ID. Allowed only for valid status transitions.
// @Tags Maintenances
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Success 204 "Maintenance started"
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request or forbidden status transition"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/maintenances/{id}/start [post]
func (i *Implementation) StartMaint(c echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.StartMaint")
	defer span.End()
	op := "start maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	err = i.maintSrv.Start(ctx, &entity.StartMaintenanceCmd{MaintID: maintID})
	if err != nil {
		xlog.Error(ctx, "start maintenance failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	return c.NoContent(http.StatusNoContent)
}

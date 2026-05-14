package apimaint

import (
	"net/http"

	"github.com/labstack/echo/v5"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/xlog"
)

// CancelMaintReasons godoc
// @Summary List maintenance cancel reasons
// @Description Returns all supported reasons that can be used when canceling a maintenance.
// @Tags Maintenances
// @Produce json
// @Success 200 {array} apimodels.MaintenanceCancelReason
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Security BearerAuth
// @Router /api/v1/maintenances/cancel-reasons [get]
func (i *Implementation) CancelMaintReasons(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.CancelMaintReasons")
	defer span.End()
	op := "get cancel maintenance reasons"
	_ = ctx
	_ = op

	return c.JSON(http.StatusOK, []apimodels.MaintenanceCancelReason{
		apimodels.MaintenanceCancelReasonConflict,
		apimodels.MaintenanceCancelReasonIncident,
		apimodels.MaintenanceCancelReasonBusinessDecision,
		apimodels.MaintenanceCancelReasonRescheduled,
		apimodels.MaintenanceCancelReasonMistake,
	})
}

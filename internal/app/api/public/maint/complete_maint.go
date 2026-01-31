//nolint:dupl // to skip 1-66 lines are duplicate of `internal/app/api/maint/start_maint.go:1-66`
package apimaint

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"

	"github.com/ruko1202/maintmode/internal/apperr"
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

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse uuid failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"id must be a valid UUID",
		))
	}

	err = i.maintSrv.Complete(ctx, &entity.CompleteMaintenanceCmd{MaintID: maintID})
	if err != nil {
		xlog.Error(ctx, "complete maintenance failed", zap.Error(err))
		if errors.Is(err, apperr.ErrForbiddenStatusTransition) {
			return c.JSON(http.StatusConflict, apierrors.NewErrorResponse(
				apierrors.ErrForbiddenStatusTransition,
				apperr.ErrForbiddenStatusTransition.Error(),
			))
		}

		if errors.Is(err, apperr.ErrMaintNotFound) {
			return c.JSON(http.StatusNotFound, apierrors.NewErrorResponse(
				apierrors.ErrNotFound,
				fmt.Sprintf("maintenance '%s' not found", maintID),
			))
		}

		return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"complete maintenance failed",
		))
	}

	return c.NoContent(http.StatusNoContent)
}

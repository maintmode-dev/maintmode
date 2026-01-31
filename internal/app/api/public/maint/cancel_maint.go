//nolint:dupl // to skip 1-66 lines are duplicate of `internal/app/api/maint/start_maint.go:1-66`
package apimaint

import (
	"errors"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// CancelMaint godoc
// @Summary Cancel maintenance
// @Description Cancel a maintenance by ID. Allowed only for valid status transitions.
// @Tags Maintenances
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Param request body apimodels.CancelMaintRequest true "Update maintenance draft request"
// @Success 204 "Maintenance canceled"
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request or forbidden status transition"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/maintenances/{id}/cancel [post]
func (i *Implementation) CancelMaint(c echo.Context) error {
	ctx := xlog.WithOperation(c.Request().Context(), "api.Maint.CancelMaint")

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse uuid failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"id must be a valid UUID",
		))
	}

	req := new(apimodels.CancelMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"cannot parse request body",
		))
	}

	if err := validateCancelRequest(req); err != nil {
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			err.Error(),
		))
	}

	reason, err := apimodels.FromAPIMaintenanceCancelReason(req.Reason)
	if err != nil {
		xlog.Error(ctx, "unsupported cancel reason", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			err.Error(),
		))
	}

	err = i.maintSrv.Cancel(ctx, &entity.CancelMaintenanceCmd{
		MaintID:       maintID,
		Reason:        reason,
		ReasonComment: req.Comment,
	})
	if err != nil {
		xlog.Error(ctx, "cancel maintenance failed", zap.Error(err))
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
			"cancel maintenance failed",
		))
	}

	return c.NoContent(http.StatusNoContent)
}

func validateCancelRequest(req *apimodels.CancelMaintRequest) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Reason, validation.Required),
		validation.Field(&req.Comment, validation.Required),
	)
}

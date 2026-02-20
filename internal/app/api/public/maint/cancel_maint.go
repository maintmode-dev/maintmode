//nolint:dupl // to skip 1-66 lines are duplicate of `internal/app/api/maint/start_maint.go:1-66`
package apimaint

import (
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
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
	op := "cancel maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse uuid failed", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	req := new(apimodels.CancelMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrParseBody)
		return c.JSON(statusCode, errResp)
	}

	if err := validateCancelRequest(req); err != nil {
		xlog.Error(ctx, "invalid request", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	reason, err := apimodels.FromAPIMaintenanceCancelReason(req.Reason)
	if err != nil {
		xlog.Error(ctx, "unsupported cancel reason", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	err = i.maintSrv.Cancel(ctx, &entity.CancelMaintenanceCmd{
		MaintID:       maintID,
		Reason:        reason,
		ReasonComment: req.Comment,
	})
	if err != nil {
		xlog.Error(ctx, "cancel maintenance failed", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	return c.NoContent(http.StatusNoContent)
}

func validateCancelRequest(req *apimodels.CancelMaintRequest) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Reason, validation.Required),
		validation.Field(&req.Comment, validation.Required),
	)
}

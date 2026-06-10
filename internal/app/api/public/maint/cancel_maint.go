//nolint:dupl // to skip 1-66 lines are duplicate of `internal/app/api/maint/start_maint.go:1-66`
package apimaint

import (
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// CancelMaint godoc
// @Summary Cancel maintenance
// @Description Cancel a maintenance by ID. Allowed only for valid status transitions.
// @Tags Maintenances
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Param request body apimodels.CancelMaintRequest true "Cancel maintenance request"
// @Success 204 "Maintenance canceled"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 409 {object} httperrors.ErrorResponse "Forbidden status transition"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/maintenances/{id}/cancel [post]
func (i *Implementation) CancelMaint(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.CancelMaint")
	defer span.End()
	op := "cancel maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	req := new(apimodels.CancelMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if err := validateCancelRequest(req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	reason, err := apimodels.FromAPIMaintenanceCancelReason(req.Reason)
	if err != nil {
		xlog.Error(ctx, "unsupported cancel reason", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	err = i.maintSrv.CancelMaint(ctx, &entity.CancelMaintenanceCmd{
		MaintID:       maintID,
		Reason:        reason,
		ReasonComment: req.Comment,
	})
	if err != nil {
		xlog.Error(ctx, "cancel maintenance failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func validateCancelRequest(req *apimodels.CancelMaintRequest) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Reason, validation.Required),
	)
}

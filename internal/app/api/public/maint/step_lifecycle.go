//nolint:dupl // no duplicate of `internal/app/api/maint/step_lifecycle.go:1-96`
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

// StartStep godoc
// @Summary Start maintenance step
// @Description Starts a maintenance step by step_id. Allowed only for valid step status transitions and step order.
// @Tags Maintenances
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Param step_id path string true "Step ID" Format(uuid)
// @Success 204 "Step started"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid UUID"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Maintenance or step not found"
// @Failure 409 {object} httperrors.ErrorResponse "Forbidden step status transition or step order violation or maintenance not in progress"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/maintenances/{id}/steps/{step_id}/start [post]
func (i *Implementation) StartStep(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.StartStep")
	defer span.End()
	op := "start maintenance step"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}
	stepID, err := uuid.Parse(c.Param("step_id"))
	if err != nil {
		xlog.Error(ctx, "parse stepID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	err = i.maintSrv.StartStep(ctx, &entity.StartMaintenanceStepCmd{MaintID: maintID, StepID: stepID})
	if err != nil {
		xlog.Error(ctx, "start maintenance step failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// CompleteStep godoc
// @Summary Complete maintenance step
// @Description Completes a maintenance step by step_id. Allowed only for valid step status transitions.
// @Tags Maintenances
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Param step_id path string true "Step ID" Format(uuid)
// @Success 204 "Step completed"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid UUID"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Maintenance or step not found"
// @Failure 409 {object} httperrors.ErrorResponse "Forbidden step status transition or maintenance not in progress"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/maintenances/{id}/steps/{step_id}/complete [post]
func (i *Implementation) CompleteStep(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.CompleteStep")
	defer span.End()
	op := "complete maintenance step"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}
	stepID, err := uuid.Parse(c.Param("step_id"))
	if err != nil {
		xlog.Error(ctx, "parse stepID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	err = i.maintSrv.CompleteStep(ctx, &entity.CompleteMaintenanceStepCmd{MaintID: maintID, StepID: stepID})
	if err != nil {
		xlog.Error(ctx, "complete maintenance step failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// CancelStep godoc
// @Summary Cancel maintenance step
// @Description Cancels a maintenance step by step_id. Allowed only for valid step status transitions.
// @Tags Maintenances
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Param step_id path string true "Step ID" Format(uuid)
// @Success 204 "Step canceled"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid UUID"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Maintenance or step not found"
// @Failure 409 {object} httperrors.ErrorResponse "Forbidden step status transition or maintenance not in progress"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/maintenances/{id}/steps/{step_id}/cancel [post]
func (i *Implementation) CancelStep(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.CancelStep")
	defer span.End()
	op := "cancel maintenance step"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}
	stepID, err := uuid.Parse(c.Param("step_id"))
	if err != nil {
		xlog.Error(ctx, "parse stepID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	err = i.maintSrv.CancelStep(ctx, &entity.CancelMaintenanceStepCmd{MaintID: maintID, StepID: stepID})
	if err != nil {
		xlog.Error(ctx, "cancel maintenance step failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

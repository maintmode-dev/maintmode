package uicalendar

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"

	uimodels "github.com/ruko1202/maintmode/internal/app/api/ui/calendar/models"
	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
)

// MaintView godoc
// @Summary Get maintenance view
// @Description Returns aggregated maintenance view for calendar and UI, including conflicts and available actions.
// @Tags UI
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Success 200 {object} uimodels.MaintenanceViewResponse
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request"
// @Failure 404 {object} apierrors.ErrorResponse "Maintenance not found"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /ui/v1/maintenances/{id} [get]
func (i *Implementation) MaintView(c echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Calendar.MaintView")
	defer span.End()

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"id must be a valid UUID",
		))
	}

	maint, err := i.calendarSrv.GetMaint(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "get maintenance failed", xfield.Error(err))
		if errors.Is(err, apperr.ErrMaintNotFound) {
			return c.JSON(http.StatusNotFound, apierrors.NewErrorResponse(
				apierrors.ErrInvalidRequest,
				err.Error(),
			))
		}

		return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
			apierrors.ErrInternalError,
			"get maintenance failed",
		))
	}

	conflicts, err := i.calendarSrv.GetConflicts(ctx, &calendardto.ConflictQueryCmd{
		MaintID:       maintID,
		PlannedPeriod: maint.PlannedPeriod,
		Scope:         maint.Scope,
		ResourceIDs: lo.Map(maint.Resources, func(item *calendardto.MaintenanceResource, _ int) uuid.UUID {
			return item.ID
		}),
	})
	if err != nil {
		xlog.Error(ctx, "get maintenance conflict failed", xfield.Error(err))
		if errors.Is(err, apperr.ErrMaintNotFound) {
			return c.JSON(http.StatusNotFound, apierrors.NewErrorResponse(
				apierrors.ErrInvalidRequest,
				err.Error(),
			))
		}

		return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
			apierrors.ErrInternalError,
			"get maintenance conflict failed",
		))
	}

	return c.JSON(http.StatusOK, &uimodels.MaintenanceViewResponse{
		Maintenance: uimodels.ToAPIMaintenanceView(maint),
		Conflicts: lo.Map(conflicts, func(item *calendardto.Conflict, _ int) *uimodels.ConflictView {
			return uimodels.ToAPIConflictView(item)
		}),
		Actions: resolveActions(maint),
	})
}

func resolveActions(m *calendardto.Maintenance) *uimodels.MaintenanceActions {
	switch m.Status {
	case entity.MaintenanceStatusDraft:
		return &uimodels.MaintenanceActions{
			CanEdit:    true,
			CanApprove: true,
			CanCancel:  true,
		}
	case entity.MaintenanceStatusPlanned:
		return &uimodels.MaintenanceActions{
			CanStart:  true,
			CanCancel: true,
		}
	case entity.MaintenanceStatusInProgress:
		return &uimodels.MaintenanceActions{
			CanFinish: true,
			CanCancel: true,
		}
	case entity.MaintenanceStatusCancelled, entity.MaintenanceStatusCompleted:
		return &uimodels.MaintenanceActions{}
	default:
		return &uimodels.MaintenanceActions{}
	}
}

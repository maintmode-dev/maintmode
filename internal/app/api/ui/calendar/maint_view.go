package uicalendar

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/utils/xecho"

	uimodels "github.com/ruko1202/maintmode/internal/app/api/ui/calendar/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/authz"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
)

// MaintView godoc
// @Summary Get maintenance view
// @Description Returns aggregated maintenance view for calendar and UI, including conflicts and available actions.
// @Tags UI
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Success 200 {object} uimodels.MaintenanceViewResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Maintenance not found"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /ui/v1/maintenances/{id} [get]
func (i *Implementation) MaintView(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Calendar.MaintView")
	defer span.End()

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return c.JSON(http.StatusBadRequest, httperrors.NewErrorResponse(
			httperrors.ErrInvalidRequest,
			"id must be a valid UUID",
		))
	}

	maint, err := i.calendarSrv.GetMaint(ctx, maintID)
	if err != nil {
		xlog.Error(ctx, "get maintenance failed", xfield.Error(err))
		if errors.Is(err, apperr.ErrMaintNotFound) {
			return c.JSON(http.StatusNotFound, httperrors.NewErrorResponse(
				httperrors.ErrInvalidRequest,
				err.Error(),
			))
		}

		return c.JSON(http.StatusInternalServerError, httperrors.NewErrorResponse(
			httperrors.ErrInternalError,
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
			return c.JSON(http.StatusNotFound, httperrors.NewErrorResponse(
				httperrors.ErrInvalidRequest,
				err.Error(),
			))
		}

		return c.JSON(http.StatusInternalServerError, httperrors.NewErrorResponse(
			httperrors.ErrInternalError,
			"get maintenance conflict failed",
		))
	}

	user, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		// not raise error, just return empty actions (user can view maint but can't do anything)'
		xlog.Error(ctx, "user not found in context")
		user = &entity.User{}
	}

	return c.JSON(http.StatusOK, &uimodels.MaintenanceViewResponse{
		Maintenance: uimodels.ToAPIMaintenanceView(maint),
		Conflicts: lo.Map(conflicts, func(item *calendardto.Conflict, _ int) *uimodels.ConflictView {
			return uimodels.ToAPIConflictView(item)
		}),
		Actions: i.resolveActions(ctx, user.Roles, maint),
	})
}

func (i *Implementation) resolveActions(ctx context.Context, roles []entity.Role, m *calendardto.Maintenance) *uimodels.MaintenanceActions {
	switch m.Status {
	case entity.MaintenanceStatusDraft:
		return &uimodels.MaintenanceActions{
			CanEdit:    can(ctx, i.authorizer, roles, entity.AuthzScenarioMaintenanceEdit),
			CanApprove: can(ctx, i.authorizer, roles, entity.AuthzScenarioMaintenanceApprove),
			CanCancel:  can(ctx, i.authorizer, roles, entity.AuthzScenarioMaintenanceCancel),
		}
	case entity.MaintenanceStatusPlanned:
		return &uimodels.MaintenanceActions{
			CanStart:  can(ctx, i.authorizer, roles, entity.AuthzScenarioMaintenanceStart),
			CanCancel: can(ctx, i.authorizer, roles, entity.AuthzScenarioMaintenanceCancel),
		}
	case entity.MaintenanceStatusInProgress:
		userCanComplete := can(ctx, i.authorizer, roles, entity.AuthzScenarioMaintenanceComplete)
		return &uimodels.MaintenanceActions{
			CanFinish: userCanComplete && allStepsTerminal(m.Steps),
			CanCancel: can(ctx, i.authorizer, roles, entity.AuthzScenarioMaintenanceCancel),
		}
	case entity.MaintenanceStatusCancelled, entity.MaintenanceStatusCompleted:
		return &uimodels.MaintenanceActions{}
	default:
		return &uimodels.MaintenanceActions{}
	}
}

func can(ctx context.Context, authorizer *authz.CasbinAuthorizer, roles []entity.Role, scenario entity.AuthzScenario) bool {
	allow, err := authorizer.Allow(ctx, roles, scenario)
	if err != nil {
		// not raise error, just return FALSE (user can view maint but can't do anything)'
		xlog.Error(ctx, "check authz failed", xfield.Error(err))
		return false
	}
	return allow
}

func allStepsTerminal(steps []*calendardto.MaintenanceStep) bool {
	for _, step := range steps {
		if !step.Status.IsTerminal() {
			return false
		}
	}
	return true
}

package apimaint

import (
	"context"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// UpdateDraftMaint godoc
// @Summary Update maintenance draft
// @Description Updates an existing maintenance draft from planned_start and steps.
// @Tags Maintenances
// @Accept json
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Param request body apimodels.UpdateDraftMaintRequest true "Update maintenance draft request"
// @Success 204 "Maintenance draft updated"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/maintenances/{id}/edit [post]
func (i *Implementation) UpdateDraftMaint(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.UpdateDraftMaint")
	defer span.End()
	op := "update maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	req := new(apimodels.UpdateDraftMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if err := validateUpdateMaintRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	cmd, err := toUpdateMaintenanceCmd(ctx, maintID, req)
	if err != nil {
		xlog.Error(ctx, "to update maintenance command failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	actor, actorErr := i.actorFromCtx(c, op)
	if actorErr != nil {
		return actorErr
	}
	cmd.Actor = actor

	if err := i.maintSrv.UpdateMaint(ctx, cmd); err != nil {
		xlog.Error(ctx, "update maintenance failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func toUpdateMaintenanceCmd(ctx context.Context, maintID uuid.UUID, req *apimodels.UpdateDraftMaintRequest) (*entity.UpdateMaintenanceCmd, error) {
	scope, err := apimodels.FromAPIScope(req.Scope)
	if err != nil {
		xlog.Error(ctx, "unsupported scope", xfield.Error(err))
		return nil, fmt.Errorf("unsupported scope")
	}

	impact, err := apimodels.FromAPIImpact(req.Impact)
	if err != nil {
		xlog.Error(ctx, "unsupported impact", xfield.Error(err))
		return nil, fmt.Errorf("unsupported impact")
	}

	steps, err := apimodels.FromAPISteps(req.Steps)
	if err != nil {
		xlog.Error(ctx, "unsupported step", xfield.Error(err))
		return nil, fmt.Errorf("unsupported step")
	}

	notifyTargets, err := apimodels.FromAPINotifyTargets(req.NotifyTargets)
	if err != nil {
		xlog.Error(ctx, "invalid notify targets", xfield.Error(err))
		return nil, fmt.Errorf("invalid notify targets")
	}

	cmd := &entity.UpdateMaintenanceCmd{
		MaintID:               maintID,
		Title:                 lo.ToPtr(req.Title),
		Description:           lo.ToPtr(req.Description),
		PlannedStart:          lo.ToPtr(req.PlannedStart),
		Scope:                 lo.ToPtr(scope),
		Impact:                lo.ToPtr(impact),
		Resources:             apimodels.FromAPIResources(req.Resources),
		Steps:                 steps,
		NotifyTargets:         notifyTargets,
		DeferredNotifications: apimodels.FromAPIDeferredNotifications(req.DeferredNotifications),
		ApproverUserID:        req.ApproverUserID,
	}

	return cmd, nil
}

//nolint:dupl // create vs update are separate by design; see validateCreateMaintDraftRequest
func validateUpdateMaintRequest(ctx context.Context, r *apimodels.UpdateDraftMaintRequest) error {
	return validation.ValidateStructWithContext(ctx, r,
		validation.Field(&r.Title, validation.Required),
		validation.Field(&r.Description, validation.Required),
		validation.Field(&r.PlannedStart, validation.Required),
		validation.Field(&r.Scope, validation.Required),
		validation.Field(&r.Impact, validation.Required),
		validation.Field(&r.Resources, validation.Required.
			When(r.Scope == apimodels.MaintenanceScopeResources),
			validation.Each(validation.WithContext(validateResource)),
		),
		validation.Field(&r.Steps, validation.Required, validation.Each(validation.WithContext(validateStep))),
		validation.Field(&r.NotifyTargets, validation.Required, validation.WithContext(validateNotifyTargets)),
		validation.Field(&r.DeferredNotifications, validation.Each(validation.WithContext(validateDeferredNotification))),
	)
}

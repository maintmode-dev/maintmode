package apimaint

import (
	"context"
	"fmt"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xvalidation"
)

// CreateDraftMaint godoc
// @Summary Create maintenance draft
// @Description Creates a maintenance in draft status from planned_start and steps.
// @Tags Maintenances
// @Accept json
// @Produce json
// @Param request body apimodels.CreateDraftMaintRequest true "Create maintenance draft request"
// @Success 200 {object} apimodels.CreateDraftMaintResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/maintenances/create [post]
func (i *Implementation) CreateDraftMaint(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.CreateDraftMaint")
	defer span.End()
	op := "create maintenance"

	req := new(apimodels.CreateDraftMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if err := validateCreateMaintDraftRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	cmd, err := toCreateMaintenanceCmd(ctx, req)
	if err != nil {
		xlog.Error(ctx, "to create maintenance command failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	maint, err := i.maintSrv.CreateDraft(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "create maintenances failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, &apimodels.CreateDraftMaintResponse{
		ID:            maint.ID,
		Title:         maint.Title,
		Description:   maint.Description,
		PlannedPeriod: apimodels.ToAPIPeriod(maint.PlannedPeriod),
		Resources:     apimodels.ToAPIResources(maint.Resources),
		Scope:         string(maint.Scope),
		Impact:        string(maint.Impact),
		Status:        string(maint.Status),
		CreatedAt:     maint.CreatedAt,
		Steps:         apimodels.ToAPISteps(maint.Steps),
		NotifyTargets: apimodels.ToAPINotifyTargets(maint.NotifyTargets),
	})
}

func toCreateMaintenanceCmd(ctx context.Context, req *apimodels.CreateDraftMaintRequest) (*entity.CreateMaintenanceCmd, error) {
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

	return &entity.CreateMaintenanceCmd{
		Title:         req.Title,
		Description:   req.Description,
		PlannedPeriod: recalculatePlannedPeriod(req.PlannedStart, steps),
		Scope:         scope,
		Impact:        impact,
		Resources:     apimodels.FromAPIResources(req.Resources),
		Steps:         steps,
		NotifyTargets: apimodels.FromAPINotifyTargets(req.NotifyTargets),
	}, nil
}

func recalculatePlannedPeriod(plannedPeriodStart time.Time, steps []*entity.MaintenanceStepInput) entity.Period {
	totalDuration := lo.SumBy(steps, func(item *entity.MaintenanceStepInput) time.Duration {
		return time.Duration(item.DurationMinutes) * time.Minute
	})
	if totalDuration == 0 {
		return entity.NewOpenEndedPeriod(plannedPeriodStart)
	}

	return entity.NewPeriod(plannedPeriodStart, plannedPeriodStart.Add(totalDuration))
}

// Create and update draft validation are intentionally kept as two
// separate functions even though they currently share the same field
// set: they validate distinct operations and their rules are expected
// to diverge (e.g. partial updates making some fields optional).
//
//nolint:dupl // see comment above — create vs update are separate by design
func validateCreateMaintDraftRequest(ctx context.Context, r *apimodels.CreateDraftMaintRequest) error {
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
		validation.Field(&r.Steps, validation.Required,
			validation.Length(1, 100),
			validation.Each(validation.WithContext(validateStep)),
		),
		validation.Field(&r.NotifyTargets, validation.Required, validation.WithContext(validateNotifyTargets)),
	)
}

func validateResource(ctx context.Context, value any) error {
	resource, err := xvalidation.Parse[apimodels.ResourceRef](value)
	if err != nil {
		return err
	}

	return validation.ValidateStructWithContext(ctx, resource,
		validation.Field(&resource.ID, validation.Required, validation.By(xvalidation.UUIDNotNil)),
	)
}

func validateStep(ctx context.Context, value any) error {
	step, err := xvalidation.Parse[apimodels.MaintenanceStepInput](value)
	if err != nil {
		return err
	}

	return validation.ValidateStructWithContext(ctx, step,
		validation.Field(&step.Description, validation.Required),
		validation.Field(&step.RollbackDescription, validation.Required),
		validation.Field(&step.Duration, validation.Required, validation.WithContext(xvalidation.IsDuration)),
	)
}

func validateNotifyTargets(ctx context.Context, value any) error {
	targets, err := xvalidation.Parse[apimodels.NotifyTargets](value)
	if err != nil {
		return err
	}

	return validation.ValidateStructWithContext(ctx, targets,
		validation.Field(&targets.ChannelIDs, validation.Required,
			validation.Length(1, 100),
			validation.Each(validation.Required),
		),
	)
}

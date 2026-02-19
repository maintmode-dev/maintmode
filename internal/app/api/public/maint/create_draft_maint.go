package apimaint

import (
	"context"
	"fmt"
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

// CreateDraftMaint godoc
// @Summary Create maintenance draft
// @Description Creates a maintenance in draft status with planned period and resources
// @Tags Maintenances
// @Accept json
// @Produce json
// @Param request body apimodels.CreateDraftMaintRequest true "Create maintenance draft request"
// @Success 200 {object} apimodels.CreateDraftMaintResponse
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/maintenances/create [post]
func (i *Implementation) CreateDraftMaint(c echo.Context) error {
	ctx := xlog.WithOperation(c.Request().Context(), "api.Maint.CreateDraftMaint")

	req := new(apimodels.CreateDraftMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"cannot parse request body",
		))
	}

	if err := validateCreateMaintDraftRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			err.Error(),
		))
	}

	cmd, err := toCreateMaintenanceCmd(ctx, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			err.Error(),
		))
	}

	maint, err := i.maintSrv.CreateDraft(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "create maintenances failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
			apierrors.ErrCreateMaint,
			"create maintenances failed",
		))
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
	})
}

func toCreateMaintenanceCmd(ctx context.Context, req *apimodels.CreateDraftMaintRequest) (*entity.CreateMaintenanceCmd, error) {
	scope, err := apimodels.FromAPIScope(req.Scope)
	if err != nil {
		xlog.Error(ctx, "unsupported scope", zap.Error(err))
		return nil, fmt.Errorf("unsupported scope")
	}

	impact, err := apimodels.FromAPIImpact(req.Impact)
	if err != nil {
		xlog.Error(ctx, "unsupported impact", zap.Error(err))
		return nil, fmt.Errorf("unsupported impact")
	}

	resources, err := apimodels.FromAPIResources(req.Resources)
	if err != nil {
		xlog.Error(ctx, "unsupported resource type", zap.Error(err))
		return nil, fmt.Errorf("unsupported resource type")
	}

	return &entity.CreateMaintenanceCmd{
		Title:         req.Title,
		Description:   req.Description,
		PlannedPeriod: apimodels.FromAPIPeriod(req.PlannedPeriod),
		Scope:         scope,
		Impact:        impact,
		Resources:     resources,
	}, nil
}

func validateCreateMaintDraftRequest(ctx context.Context, r *apimodels.CreateDraftMaintRequest) error {
	return validation.ValidateStructWithContext(ctx, r,
		validation.Field(&r.Title, validation.Required),
		validation.Field(&r.Description, validation.Required),
		validation.Field(&r.PlannedPeriod, validation.Required),
		validation.Field(&r.Scope, validation.Required),
		validation.Field(&r.Impact, validation.Required),
		validation.Field(&r.Resources, validation.Required.
			When(r.Scope == apimodels.MaintenanceScopeResources),
			validation.Each(validation.WithContext(validateResource)),
		),
	)
}

func validateResource(ctx context.Context, value any) error {
	var resource *apimodels.Resource
	switch v := value.(type) {
	case *apimodels.Resource:
		resource = v
	case apimodels.Resource:
		resource = &v
	default:
		return fmt.Errorf("unsupported resource type: %T", v)
	}

	return validation.ValidateStructWithContext(ctx, resource,
		validation.Field(&resource.ID, validation.Required, validation.By(uuidNotZero)),
		validation.Field(&resource.Type, validation.Required),
	)
}

func uuidNotZero(value any) error {
	if value == uuid.Nil {
		return fmt.Errorf("id cannot be zero")
	}
	return nil
}

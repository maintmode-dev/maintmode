package apimaint

import (
	"context"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

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
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.CreateDraftMaint")
	defer span.End()
	op := "create maintenance"

	req := new(apimodels.CreateDraftMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	if err := validateCreateMaintDraftRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	cmd, err := toCreateMaintenanceCmd(ctx, req)
	if err != nil {
		xlog.Error(ctx, "to create maintenance command failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	maint, err := i.maintSrv.CreateDraft(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "create maintenances failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
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
		xlog.Error(ctx, "unsupported scope", xfield.Error(err))
		return nil, fmt.Errorf("unsupported scope")
	}

	impact, err := apimodels.FromAPIImpact(req.Impact)
	if err != nil {
		xlog.Error(ctx, "unsupported impact", xfield.Error(err))
		return nil, fmt.Errorf("unsupported impact")
	}

	resources, err := apimodels.FromAPIResources(req.Resources)
	if err != nil {
		xlog.Error(ctx, "unsupported resource type", xfield.Error(err))
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

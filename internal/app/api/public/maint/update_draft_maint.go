package apimaint

import (
	"context"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// UpdateDraftMaint godoc
// @Summary Update maintenance draft
// @Description Updates an existing maintenance draft by ID.
// @Tags Maintenances
// @Accept json
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Param request body apimodels.UpdateDraftMaintRequest true "Update maintenance draft request"
// @Success 204 "Maintenance draft updated"
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/maintenances/{id}/edit [post]
func (i *Implementation) UpdateDraftMaint(c echo.Context) error {
	ctx := xlog.WithOperation(c.Request().Context(), "api.Maint.UpdateDraftMaint")
	op := "update maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse uuid failed", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	req := new(apimodels.UpdateDraftMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrParseBody)
		return c.JSON(statusCode, errResp)
	}

	if err := validateUpdateMaintRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	cmd, err := toUpdateMaintenanceCmd(ctx, maintID, req)
	if err != nil {
		xlog.Error(ctx, "to update maintenance command failed", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	if err := i.maintSrv.Update(ctx, cmd); err != nil {
		xlog.Error(ctx, "update maintenance failed", zap.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	return c.NoContent(http.StatusNoContent)
}

func toUpdateMaintenanceCmd(ctx context.Context, maintID uuid.UUID, req *apimodels.UpdateDraftMaintRequest) (*entity.UpdateMaintenanceCmd, error) {
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

	return &entity.UpdateMaintenanceCmd{
		MaintID:       maintID,
		Title:         lo.ToPtr(req.Title),
		Description:   lo.ToPtr(req.Description),
		PlannedPeriod: lo.ToPtr(apimodels.FromAPIPeriod(req.PlannedPeriod)),
		Scope:         lo.ToPtr(scope),
		Impact:        lo.ToPtr(impact),
		Resources:     resources,
	}, nil
}

func validateUpdateMaintRequest(ctx context.Context, r *apimodels.UpdateDraftMaintRequest) error {
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

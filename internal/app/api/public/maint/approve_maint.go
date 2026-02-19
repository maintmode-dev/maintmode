package apimaint

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// ApproveMaint godoc
// @Summary Approve maintenance draft
// @Description Approves a maintenance draft by ID using an observed revision and conflict snapshot.
// @Tags Maintenances
// @Accept json
// @Produce json
// @Param id path string true "Maintenance ID" Format(uuid)
// @Param request body apimodels.ApproveDraftMaintRequest true "Approve maintenance request"
// @Success 204 "Maintenance approved"
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request or forbidden status transition"
// @Failure 404 {object} apierrors.ErrorResponse "Maintenance not found"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/maintenances/{id}/approve [post]
func (i *Implementation) ApproveMaint(c echo.Context) error {
	ctx := xlog.WithOperation(c.Request().Context(), "api.Maint.ApproveMaint")

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse uuid failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"id must be a valid UUID",
		))
	}

	req := new(apimodels.ApproveDraftMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"cannot parse request body",
		))
	}

	if err := validateApproveDraftMaintRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			err.Error(),
		))
	}

	cmd, err := toApproveMaintenanceCmd(ctx, maintID, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			err.Error(),
		))
	}

	err = i.maintSrv.Approve(ctx, cmd)
	//nolint:nestif
	if err != nil {
		xlog.Error(ctx, "approve maintenance failed", zap.Error(err))
		if errors.Is(err, apperr.ErrForbiddenStatusTransition) {
			return c.JSON(http.StatusConflict, apierrors.NewErrorResponse(
				apierrors.ErrForbiddenStatusTransition,
				apperr.ErrForbiddenStatusTransition.Error(),
			))
		}

		if errors.Is(err, apperr.ErrConflictsChangedSincePreview) {
			return c.JSON(http.StatusConflict, apierrors.NewErrorResponse(
				apierrors.ErrConflictsChangedSincePreview,
				err.Error(),
			))
		}

		if errors.Is(err, apperr.ErrMaintChangedSincePreview) {
			return c.JSON(http.StatusConflict, apierrors.NewErrorResponse(
				apierrors.ErrMaintChangedSincePreview,
				err.Error(),
			))
		}

		if errors.Is(err, apperr.ErrMaintNotFound) {
			return c.JSON(http.StatusNotFound, apierrors.NewErrorResponse(
				apierrors.ErrNotFound,
				fmt.Sprintf("maintenance '%s' not found", maintID),
			))
		}

		return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"approve maintenance failed",
		))
	}

	return c.NoContent(http.StatusNoContent)
}

func validateApproveDraftMaintRequest(ctx context.Context, req *apimodels.ApproveDraftMaintRequest) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.ObservedMaintRevision, validation.Required),
		validation.Field(&req.ConflictsSnapshot, validation.Each(validation.WithContext(validateConflicts))),
	)
}

func validateConflicts(ctx context.Context, value any) error {
	var conflict *apimodels.Conflict
	switch v := value.(type) {
	case *apimodels.Conflict:
		conflict = v
	case apimodels.Conflict:
		conflict = &v
	default:
		return fmt.Errorf("unsupported resource type: %T", v)
	}

	return validation.ValidateStructWithContext(ctx, conflict,
		validation.Field(&conflict.MaintenanceID, validation.Required, validation.By(uuidNotZero)),
		validation.Field(&conflict.OverlapStart, validation.Required),
		validation.Field(&conflict.OverlapEnd, validation.Required),
		validation.Field(&conflict.Scope, validation.Required),
		validation.Field(&conflict.Resources, validation.Required.
			When(conflict.Scope == apimodels.MaintenanceScopeResources),
			validation.Each(validation.WithContext(validateResource)),
		),
	)
}

func toApproveMaintenanceCmd(ctx context.Context, maintID uuid.UUID, req *apimodels.ApproveDraftMaintRequest) (*entity.ApproveMaintenanceCmd, error) {
	conflictsSnapshot := make([]*entity.ConflictWithResources, 0, len(req.ConflictsSnapshot))
	for _, conflict := range req.ConflictsSnapshot {
		resources, err := apimodels.FromAPIResources(conflict.Resources)
		if err != nil {
			xlog.Error(ctx, "unsupported resource type", zap.Error(err))
			return nil, fmt.Errorf("unsupported resource type")
		}

		scope, err := apimodels.FromAPIScope(conflict.Scope)
		if err != nil {
			xlog.Error(ctx, "unsupported scope", zap.Error(err))
			return nil, fmt.Errorf("unsupported scope")
		}

		conflictsSnapshot = append(conflictsSnapshot, &entity.ConflictWithResources{
			Resources: resources,
			Conflict: &entity.Conflict{
				MaintenanceID: conflict.MaintenanceID,
				OverlapStart:  conflict.OverlapStart,
				OverlapEnd:    conflict.OverlapEnd,
				Scope:         scope,
			},
		})
	}

	return &entity.ApproveMaintenanceCmd{
		MaintID:               maintID,
		ObservedMaintRevision: req.ObservedMaintRevision,
		ConflictSnapshot: entity.ConflictsSnapshot{
			Conflicts: conflictsSnapshot,
		},
	}, nil
}

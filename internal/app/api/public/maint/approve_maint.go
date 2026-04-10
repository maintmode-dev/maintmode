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
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.ApproveMaint")
	defer span.End()
	op := "approve maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	req := new(apimodels.ApproveDraftMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrParseBody)
		return c.JSON(statusCode, errResp)
	}

	if err := validateApproveDraftMaintRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	cmd, err := toApproveMaintenanceCmd(ctx, maintID, req)
	if err != nil {
		xlog.Error(ctx, "to approve maintenance command failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	err = i.maintSrv.Approve(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "approve maintenance failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
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
			xlog.Error(ctx, "unsupported resource type", xfield.Error(err))
			return nil, fmt.Errorf("unsupported resource type")
		}

		scope, err := apimodels.FromAPIScope(conflict.Scope)
		if err != nil {
			xlog.Error(ctx, "unsupported scope", xfield.Error(err))
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

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

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/maint/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xvalidation"
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
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Maintenance not found"
// @Failure 409 {object} httperrors.ErrorResponse "Forbidden status transition or conflicts changed since preview"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/maintenances/{id}/approve [post]
func (i *Implementation) ApproveMaint(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Maint.ApproveMaint")
	defer span.End()
	op := "approve maintenance"

	maintID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse maintID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	req := new(apimodels.ApproveDraftMaintRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if err := validateApproveDraftMaintRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	cmd, err := toApproveMaintenanceCmd(ctx, maintID, req)
	if err != nil {
		xlog.Error(ctx, "to approve maintenance command failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	err = i.maintSrv.ApproveMaint(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "approve maintenance failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
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
	conflict, err := xvalidation.Parse[apimodels.Conflict](value)
	if err != nil {
		return err
	}

	return validation.ValidateStructWithContext(ctx, conflict,
		validation.Field(&conflict.MaintenanceID, validation.Required, validation.By(xvalidation.UUIDNotNil)),
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
		resources := apimodels.FromAPIResources(conflict.Resources)

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

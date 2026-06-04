//nolint:dupl
package roles

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
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/roles/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// Assign godoc
// @Summary Assign role to user
// @Description Assigns a role to a user. Requires admin privileges.
// @Tags Roles
// @Accept json
// @Produce json
// @Param request body apimodels.AssignRoleRequest true "Assign role request"
// @Success 204 "Role assigned"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request or validation error"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/roles/assign [post]
func (i *Implementation) Assign(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Roles.Assign")
	defer span.End()
	op := "roles assign"

	req := new(apimodels.AssignRoleRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if err := validateAssignRoleRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("actor not found")
		xlog.Error(ctx, "actor user not found in echo context", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	cmd, err := toAssignRoleCmd(ctx, actor, req)
	if err != nil {
		xlog.Error(ctx, "to assign role command failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	if _, err = i.userSrv.AssignRoles(ctx, cmd); err != nil {
		xlog.Error(ctx, "assign role failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func validateAssignRoleRequest(ctx context.Context, req *apimodels.AssignRoleRequest) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.UserID, validation.Required),
		validation.Field(&req.Role, validation.Required),
	)
}

func toAssignRoleCmd(ctx context.Context, actor *entity.User, req *apimodels.AssignRoleRequest) (*entity.AssignRolesCmd, error) {
	role, err := apimodels.FromAPIRole(req.Role)
	if err != nil {
		xlog.Error(ctx, "unsupported role", xfield.Error(err))
		return nil, fmt.Errorf("unsupported role")
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		xlog.Error(ctx, "invalid user id", xfield.Error(err))
		return nil, fmt.Errorf("invalid user id")
	}

	return &entity.AssignRolesCmd{
		Actor:  actor,
		UserID: userID,
		Roles:  []entity.Role{role},
	}, nil
}

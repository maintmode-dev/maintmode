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

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/roles/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// Revoke godoc
// @Summary Revoke role from user
// @Description Revokes a role from a user. Requires admin privileges.
// @Tags Roles
// @Accept json
// @Produce json
// @Param request body apimodels.RevokeRoleRequest true "Revoke role request"
// @Success 204 "Role revoked"
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request or validation error"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/roles/revoke [post]
func (i *Implementation) Revoke(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Roles.Revoke")
	defer span.End()
	op := "roles revoke"

	req := new(apimodels.RevokeRoleRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	if err := validateRevokeRoleRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("actor user not found")
		xlog.Error(ctx, "actor user not found in echo context", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	cmd, err := toRevokeRoleCmd(ctx, actor, req)
	if err != nil {
		xlog.Error(ctx, "to revoke role command failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ValidationErr(err))
		return c.JSON(statusCode, errResp)
	}

	err = i.userSrv.RevokeRole(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "revoke role failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	return c.NoContent(http.StatusNoContent)
}

func validateRevokeRoleRequest(ctx context.Context, req *apimodels.RevokeRoleRequest) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.UserID, validation.Required),
		validation.Field(&req.Role, validation.Required),
	)
}

func toRevokeRoleCmd(ctx context.Context, actor *entity.User, req *apimodels.RevokeRoleRequest) (*entity.RevokeRoleCmd, error) {
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

	return &entity.RevokeRoleCmd{
		Actor:  actor,
		UserID: userID,
		Role:   role,
	}, nil
}

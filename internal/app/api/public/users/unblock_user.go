//nolint:dupl
package users

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// UnblockUser godoc
// @Summary Unblock user
// @Description Clears blocked_at. Roles were preserved on block, so access is immediately restored. Idempotent. Requires admin privileges.
// @Tags Users
// @Produce json
// @Param id path string true "User ID (UUID)" Format(uuid)
// @Success 204 "User unblocked"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "User not found"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/users/{id}/unblock [post]
func (i *Implementation) UnblockUser(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Users.UnblockUser")
	defer span.End()
	op := "unblock user"

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse userID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("actor not found")
		xlog.Error(ctx, "actor user not found in echo context", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	err = i.userSrv.UnblockUser(ctx, &entity.UnblockUserCmd{Actor: actor, UserID: userID})
	if err != nil {
		xlog.Error(ctx, "unblock user failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

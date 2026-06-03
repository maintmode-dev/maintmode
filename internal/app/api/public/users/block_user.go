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

// BlockUser godoc
// @Summary Block user
// @Description Blocks a user: sets blocked_at and revokes their refresh tokens. Roles are preserved. Idempotent. Requires admin privileges.
// @Tags Users
// @Produce json
// @Param id path string true "User ID (UUID)" Format(uuid)
// @Success 204 "User blocked"
// @Failure 400 {object} httperrors.ErrorResponse "Cannot block self or the last active admin"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "User not found"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/users/{id}/block [post]
func (i *Implementation) BlockUser(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Users.BlockUser")
	defer span.End()
	op := "block user"

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

	err = i.userSrv.BlockUser(ctx, &entity.BlockUserCmd{Actor: actor, UserID: userID})
	if err != nil {
		xlog.Error(ctx, "block user failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

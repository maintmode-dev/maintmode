package roles

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/roles/models"
)

// ListRoles godoc
// @Summary List user roles
// @Description Returns all roles assigned to a user by their ID.
// @Tags Roles
// @Produce json
// @Param id path string true "User ID (UUID)" Format(uuid)
// @Success 200 {object} apimodels.ListRolesResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid UUID"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/user/{id}/roles [get]
func (i *Implementation) ListRoles(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Roles.ListRoles")
	defer span.End()
	op := "list roles"

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse userID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	roles, err := i.userSrv.GetRoles(ctx, userID)
	if err != nil {
		xlog.Error(ctx, "get roles failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, &apimodels.ListRolesResponse{
		Roles: apimodels.ToAPIRoles(ctx, roles),
	})
}

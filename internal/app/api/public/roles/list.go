package roles

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/roles/models"
)

// ListRoles godoc
// @Summary List user roles
// @Description Returns all roles assigned to a user by their ID.
// @Tags Roles
// @Produce json
// @Param id path string true "User ID (UUID)" Format(uuid)
// @Success 200 {object} apimodels.ListRolesResponse
// @Failure 400 {object} apierrors.ErrorResponse "Invalid UUID"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/user/{id}/roles [get]
func (i *Implementation) ListRoles(c echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Roles.ListRoles")
	defer span.End()
	op := "list roles"

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse userID failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrInvalidUUID)
		return c.JSON(statusCode, errResp)
	}

	roles, err := i.userSrv.GetRoles(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get roles",
		})
	}
	return c.JSON(http.StatusOK, &apimodels.ListRolesResponse{
		Roles: apimodels.ToAPIRoles(ctx, roles),
	})
}

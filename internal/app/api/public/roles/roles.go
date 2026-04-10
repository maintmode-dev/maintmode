package roles

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/roles/models"
)

// AvailableRoles godoc
// @Summary List available roles
// @Description Returns all roles available in the system.
// @Tags Roles
// @Produce json
// @Success 200 {object} apimodels.ListRolesResponse
// @Router /api/v1/roles [get]
func (i *Implementation) AvailableRoles(c echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Roles.AvailableRoles")
	defer span.End()
	op := "available roles"
	_ = ctx
	_ = op

	return c.JSON(http.StatusOK, &apimodels.ListRolesResponse{
		Roles: apimodels.AllRoles,
	})
}

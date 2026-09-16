package integrationapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// Delete godoc
// @Summary Delete an integration
// @Description Removes an integration and its stored secrets. Refused with 409 while accounts still sign in through this provider name — unlink them first. Admin only.
// @Tags Integrations
// @Produce json
// @Param kind path string true "Category: notify or login"
// @Param name path string true "System within the category: slack, telegram, email, google or custom"
// @Success 204 "Deleted"
// @Failure 403 {object} httperrors.ErrorResponse
// @Failure 404 {object} httperrors.ErrorResponse
// @Failure 409 {object} httperrors.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/integrations/{kind}/{name} [delete]
func (i *Implementation) Delete(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Integration.Delete")
	defer span.End()
	op := "delete integration"

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(fmt.Errorf("actor not found")))
	}

	if err := i.integrationSrv.Delete(ctx, c.Param("kind"), c.Param("name"), actor); err != nil {
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

package authsettingsapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/authsettings/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// SetEnabled godoc
// @Summary Enable or disable a built-in sign-in method
// @Description Sets whether a built-in method is offered on the login page and accepted at sign-in. Disabling every method is allowed: it is what an SSO-only instance looks like, existing sessions are not ended, and break-glass still answers. Admin only.
// @Tags Auth
// @Accept json
// @Produce json
// @Param method path string true "Built-in method: email_otp or email_password"
// @Param request body apimodels.SetAuthMethodEnabledRequest true "Target state"
// @Success 200 {object} apimodels.AuthMethodSetting
// @Failure 400 {object} httperrors.ErrorResponse
// @Failure 403 {object} httperrors.ErrorResponse
// @Failure 404 {object} httperrors.ErrorResponse "No such built-in method"
// @Security BearerAuth
// @Router /api/v1/auth/settings/{method} [patch]
func (i *Implementation) SetEnabled(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.AuthSettings.SetEnabled")
	defer span.End()
	op := "set auth method enabled"

	req := new(apimodels.SetAuthMethodEnabledRequest)
	if err := c.Bind(req); err != nil {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	actor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(fmt.Errorf("actor not found")))
	}

	updated, err := i.settingsSrv.SetEnabled(ctx, &entity.SetAuthMethodEnabledCmd{
		Method:  entity.AuthMethodName(c.Param("method")),
		Enabled: req.Enabled,
		Actor:   actor,
	})
	if err != nil {
		xlog.Error(ctx, "set auth method enabled failed", xfield.Error(err))

		// The 404 comes from the shared mapper, where every other sentinel's
		// status lives: mapping it here would be a second source of truth for
		// the same error.
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, toAPI(updated))
}

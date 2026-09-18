package authsettingsapi

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/authsettings/models"
)

// List godoc
// @Summary List the built-in sign-in methods and whether they are enabled
// @Description Returns one entry per built-in method (email_otp, email_password) with its on/off flag. Login providers are not listed here -- they are configured through the integration registry and carry their own enabled flag. Admin only.
// @Tags Auth
// @Produce json
// @Success 200 {object} apimodels.AuthMethodSettingsResponse
// @Failure 403 {object} httperrors.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/auth/settings [get]
func (i *Implementation) List(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.AuthSettings.List")
	defer span.End()
	op := "list auth settings"

	settings, err := i.settingsSrv.List(ctx)
	if err != nil {
		xlog.Error(ctx, "list auth settings failed", xfield.Error(err))

		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apimodels.AuthMethodSettingsResponse{Methods: toAPIList(settings)})
}

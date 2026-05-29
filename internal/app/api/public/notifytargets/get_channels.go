package apinotifications

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
)

// GetChannels godoc
// @Summary List notification channels
// @Description Returns the full channel catalog across enabled
// @Description transports. Used by the admin UI to populate the channel
// @Description picker when creating or editing a maintenance.
// @Tags Notifications
// @Accept json
// @Produce json
// @Success 200 {object} apimodels.ChannelsResponse
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/notifications/channels [get]
func (i *Implementation) GetChannels(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Notifications.GetChannels")
	defer span.End()

	channels, err := i.notifyTargets.AvailableChannels(ctx)
	if err != nil {
		xlog.Error(ctx, "get available channels failed", xfield.Error(err))
		return httperrors.ToAPIError(c, "get available channels", err)
	}

	return c.JSON(http.StatusOK, apimodels.ToChannelsResponse(channels))
}

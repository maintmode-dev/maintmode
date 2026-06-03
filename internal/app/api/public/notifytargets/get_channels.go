package apinotifications

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
)

// GetChannels godoc
// @Summary List notification channels
// @Description Returns the channel catalog across enabled transports. Used by
// @Description the admin UI to populate the channel picker. Archived channels
// @Description are hidden unless include_archived=true is passed.
// @Tags Notifications
// @Accept json
// @Produce json
// @Param include_archived query boolean false "Include archived channels (default false)"
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

	// Tolerant parse: an absent or malformed flag means "active only".
	includeArchived, _ := strconv.ParseBool(c.QueryParam("include_archived"))

	channels, err := i.notifyTargets.AvailableChannels(ctx, includeArchived)
	if err != nil {
		xlog.Error(ctx, "get available channels failed", xfield.Error(err))
		return httperrors.ToAPIError(c, "get available channels", err)
	}

	return c.JSON(http.StatusOK, apimodels.ToChannelsResponse(channels))
}

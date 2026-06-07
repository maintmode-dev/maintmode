package apinotifications

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"

	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
)

// GetTransports godoc
// @Summary List supported notification transports
// @Description Returns the catalog of transports a channel can be created on
// @Description (slack, telegram, ...). Each entry carries the UX copy the
// @Description channel-create form needs: title, description and the
// @Description transport-specific transport_channel_id label, placeholder and
// @Description helper text. It is a reference catalog available to any
// @Description authenticated role, not an attestation, so it never returns 403.
// @Tags Notifications
// @Produce json
// @Success 200 {object} apimodels.TransportsResponse
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/notifications/transports [get]
func (i *Implementation) GetTransports(c *echo.Context) error {
	_, span := xlog.WithOperationSpan(c.Request().Context(), "api.Notifications.GetTransports")
	defer span.End()

	return c.JSON(http.StatusOK, apimodels.TransportsResponse{Transports: apimodels.SupportedTransports})
}

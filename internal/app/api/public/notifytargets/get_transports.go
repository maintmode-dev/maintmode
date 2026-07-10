package apinotifications

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
)

// GetTransports godoc
// @Summary List supported notification transports
// @Description Returns the catalog of transports a channel can be created on
// @Description (slack, telegram, ...). Each entry carries the UX copy the
// @Description channel-create form needs: title, description and the
// @Description transport-specific transport_channel_id label, placeholder and
// @Description helper text. Each entry also carries transport_status — whether
// @Description an enabled integration backs the transport right now — so the
// @Description form can flag transports that would silently not deliver. It is
// @Description a reference catalog available to any authenticated role, not an
// @Description attestation, so it never returns 403.
// @Tags Notifications
// @Produce json
// @Success 200 {object} apimodels.TransportsResponse
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/notifications/transports [get]
func (i *Implementation) GetTransports(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Notifications.GetTransports")
	defer span.End()
	op := "get transports"

	index, err := i.integrationIndex(ctx)
	if err != nil {
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apimodels.ToTransportsResponse(index))
}

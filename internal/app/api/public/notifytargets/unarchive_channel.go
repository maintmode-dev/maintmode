package apinotifications

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
)

// UnarchiveChannel godoc
// @Summary Unarchive a notification channel
// @Description Returns a previously archived channel to the active catalog so
// @Description it shows up in the picker again. Idempotent: unarchiving an
// @Description already-active or unknown channel succeeds. Requires the editor
// @Description role.
// @Tags Notifications
// @Produce json
// @Param id path string true "Channel ID" Format(uuid)
// @Success 204 "Channel unarchived"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid channel id"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/notifications/channels/{id}/unarchive [post]
func (i *Implementation) UnarchiveChannel(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Notifications.UnarchiveChannel")
	defer span.End()
	op := "unarchive channel"

	channelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse channelID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	if err := i.notifyTargets.UnarchiveChannel(ctx, channelID); err != nil {
		xlog.Error(ctx, "unarchive channel failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

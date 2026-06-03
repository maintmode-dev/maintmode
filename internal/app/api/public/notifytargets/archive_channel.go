package apinotifications

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
)

// ArchiveChannel godoc
// @Summary Archive a notification channel
// @Description Soft-deletes a channel: it disappears from the default
// @Description GET /channels listing but stays resolvable so existing
// @Description subscriptions keep validating. Idempotent: archiving an
// @Description already-archived or unknown channel succeeds. Requires the
// @Description editor role.
// @Tags Notifications
// @Produce json
// @Param id path string true "Channel ID" Format(uuid)
// @Success 204 "Channel archived"
// @Failure 400 {object} httperrors.ErrorResponse "Invalid channel id"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/notifications/channels/{id}/archive [post]
func (i *Implementation) ArchiveChannel(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Notifications.ArchiveChannel")
	defer span.End()
	op := "archive channel"

	channelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse channelID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	if err := i.notifyTargets.ArchiveChannel(ctx, channelID); err != nil {
		xlog.Error(ctx, "archive channel failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.NoContent(http.StatusNoContent)
}

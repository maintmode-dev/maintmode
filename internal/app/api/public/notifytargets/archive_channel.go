package apinotifications

import (
	"github.com/labstack/echo/v5"
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
	return changeArchiveState(c, "api.Notifications.ArchiveChannel", "archive channel", i.notifyTargets.ArchiveChannel)
}

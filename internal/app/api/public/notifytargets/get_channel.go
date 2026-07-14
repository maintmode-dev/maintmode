package apinotifications

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// GetChannel godoc
// @Summary Get a notification channel by ID
// @Description Returns a single channel from the catalog by its id, including
// @Description authorship metadata (created_by / updated_by) resolved from the
// @Description auth service. Archived channels are still returned so the detail
// @Description page can render them. Available to any authenticated role.
// @Tags Notifications
// @Accept json
// @Produce json
// @Param id path string true "Channel ID" Format(uuid)
// @Success 200 {object} apimodels.Channel
// @Failure 400 {object} httperrors.ErrorResponse "Invalid channel id"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 404 {object} httperrors.ErrorResponse "Channel not found"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/notifications/channels/{id} [get]
func (i *Implementation) GetChannel(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Notifications.GetChannel")
	defer span.End()
	op := "get channel"

	channelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse channelID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	channel, err := i.notifyTargets.GetChannel(ctx, channelID)
	if err != nil {
		xlog.Error(ctx, "get channel failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// Resolve author/editor profiles from auth in one batch call (degrades to a
	// labeled summary on failure; never errors the read). ResolveMany dedups and
	// drops nil ids, so author==editor or an unedited channel are safe.
	summaries := i.userSummarySrv.ResolveMany(ctx, channelUserIDs([]*entity.NotifyChannel{channel}))

	index, err := i.transportStatuses(ctx, []entity.NotifyTransport{channel.Transport})
	if err != nil {
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apimodels.ToChannel(channel,
		summaryFor(summaries, channel.CreatedByUserID),
		summaryFor(summaries, channel.UpdatedByUserID),
		index,
	))
}

// summaryFor resolves a (possibly nil) user id against the resolved summary
// index. A nil id (no author/editor) maps to nil.
func summaryFor(summaries map[uuid.UUID]*entity.UserSummary, id *uuid.UUID) *entity.UserSummary {
	if id == nil {
		return nil
	}
	return summaries[*id]
}

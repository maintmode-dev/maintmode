package apinotifications

import (
	"context"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

// UpdateChannel godoc
// @Summary Update a notification channel
// @Description Partially updates a channel (name, description,
// @Description transport_channel_id). All body fields are optional; an omitted
// @Description field is left unchanged. Transport is immutable and cannot be
// @Description changed — switching it would break notification history and
// @Description existing subscriptions, so a new channel must be created instead;
// @Description any transport key in the body is ignored. Requires the editor role.
// @Tags Notifications
// @Accept json
// @Produce json
// @Param id path string true "Channel ID" Format(uuid)
// @Param request body apimodels.UpdateChannelRequest true "Fields to update"
// @Success 200 {object} apimodels.Channel
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 404 {object} httperrors.ErrorResponse "Channel not found"
// @Failure 409 {object} httperrors.ErrorResponse "Channel already exists"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/notifications/channels/{id} [patch]
func (i *Implementation) UpdateChannel(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Notifications.UpdateChannel")
	defer span.End()
	op := "update channel"

	channelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		xlog.Error(ctx, "parse channelID failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrInvalidUUID)
	}

	req := new(apimodels.UpdateChannelRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	if err := validateUpdateChannelRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	editor, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		err := fmt.Errorf("editor not found")
		xlog.Error(ctx, "editor user not found in echo context", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	// Snapshot the transport status BEFORE the write: failing on it after the
	// update is committed would report a successful mutation as a 500.
	// Transport is immutable, so reading it pre-write is safe.
	current, err := i.notifyTargets.GetChannel(ctx, channelID)
	if err != nil {
		xlog.Error(ctx, "load channel for status failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	index, err := i.transportStatuses(ctx, []entity.NotifyTransport{current.Transport})
	if err != nil {
		return httperrors.ToAPIError(c, op, err)
	}

	channel, err := i.notifyTargets.UpdateChannel(ctx, &entity.UpdateNotifyChannelCmd{
		ID:                 channelID,
		Name:               req.Name,
		Description:        req.Description,
		TransportChannelID: req.TransportChannelID,
		UpdatedByUserID:    editor.ID,
	})
	if err != nil {
		xlog.Error(ctx, "update channel failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// Resolve only the author from auth — the editor is the authenticated caller,
	// so render its summary directly. ResolveOne degrades to a labeled summary on
	// failure and never errors the write result.
	author := i.userSummarySrv.ResolveOne(ctx, lo.FromPtr(channel.CreatedByUserID))

	return c.JSON(http.StatusOK, apimodels.ToChannel(channel, author, editor.ToUserSummary(), index))
}

func validateUpdateChannelRequest(ctx context.Context, req *apimodels.UpdateChannelRequest) error {
	// NilOrNotEmpty allows an omitted (nil) field but rejects an empty string, so
	// name / transport_channel_id can't be cleared to "". description may be set
	// to an empty string (no rule), matching the resource PATCH semantics.
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.Name, validation.NilOrNotEmpty),
		validation.Field(&req.Description, validation.NilOrNotEmpty),
		validation.Field(&req.TransportChannelID, validation.NilOrNotEmpty),
	)
}

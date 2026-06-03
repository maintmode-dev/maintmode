package apinotifications

import (
	"context"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/notifytargets/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

// CreateChannel godoc
// @Summary Create a notification channel
// @Description Registers a new channel in the catalog. The catalog is the
// @Description single source of truth for subscription validation across all
// @Description instances, so a channel created here is immediately usable on
// @Description every pod without a restart. Requires the editor role.
// @Tags Notifications
// @Accept json
// @Produce json
// @Param request body apimodels.CreateChannelRequest true "Channel details"
// @Success 201 {object} apimodels.Channel
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 409 {object} httperrors.ErrorResponse "Channel already exists"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/notifications/channels [post]
func (i *Implementation) CreateChannel(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Notifications.CreateChannel")
	defer span.End()
	op := "create channel"

	req := new(apimodels.CreateChannelRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	if err := validateCreateChannelRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	channel, err := i.notifyTargets.CreateChannel(ctx, &entity.CreateNotifyChannelCmd{
		Transport:          entity.NotifyTransport(req.Transport),
		TransportChannelID: req.TransportChannelID,
		Name:               req.Name,
		Description:        req.Description,
	})
	if err != nil {
		xlog.Error(ctx, "create channel failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusCreated, apimodels.ToChannel(channel))
}

func validateCreateChannelRequest(ctx context.Context, req *apimodels.CreateChannelRequest) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.Transport, validation.Required),
		validation.Field(&req.TransportChannelID, validation.Required),
		validation.Field(&req.Name, validation.Required),
	)
}

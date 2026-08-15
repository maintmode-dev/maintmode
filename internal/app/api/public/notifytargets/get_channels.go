package apinotifications

import (
	"net/http"
	"strconv"
	"strings"

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

// GetChannels godoc
// @Summary List notification channels
// @Description Returns a page of the channel catalog across enabled transports,
// @Description ordered by (transport, transport_channel_id). Used by the admin UI
// @Description to populate the channel picker and the catalog screen. Archived
// @Description channels are hidden unless include_archived=true is passed.
// @Description Malformed pagination/filter params are coerced to defaults rather than rejected.
// @Tags Notifications
// @Accept json
// @Produce json
// @Param name query string false "Case-insensitive partial name match (search box)"
// @Param limit query int false "Page size (max 200)" default(50)
// @Param offset query int false "Pagination offset" default(0)
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
	op := "get available channels"

	cmd := queryToListChannelsCmd(c)

	result, err := i.notifyTargets.AvailableChannels(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "get available channels failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// Batch-resolve every author/editor id across the page in one auth call
	// (ResolveMany dedups and drops nil ids; degrades to "Unknown user" on
	// failure, never erroring the read).
	summaries := i.userSummarySrv.ResolveMany(ctx, channelUserIDs(result.Channels))

	// Statuses cover the transports present on this page rather than the whole
	// catalog — a page is all this response describes.
	index, err := i.transportStatuses(ctx,
		lo.Map(result.Channels, func(ch *entity.NotifyChannel, _ int) entity.NotifyTransport { return ch.Transport }))
	if err != nil {
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK,
		apimodels.ToChannelsResponse(result.Channels, summaries, index, result.Total, cmd.Limit, cmd.Offset))
}

// queryToListChannelsCmd parses the pagination and filter params. Malformed
// values are coerced to defaults rather than rejected: this is a read-only
// reference list, so a best-effort response beats a 400.
func queryToListChannelsCmd(c *echo.Context) *entity.ListChannelsCmd {
	// PagingParams is called without options — its defaults (50, max 200, offset
	// ceiling) are exactly this endpoint's contract. The error is deliberately
	// dropped: the returned Paging is valid either way, and it only reports that
	// a value was unparseable. Audit turns the same error into a 400; this
	// endpoint must not.
	paging, _ := xecho.PagingParams(c)

	// Trimmed here so the store sees "" for a whitespace-only query and skips
	// the filter entirely. Guarding on the raw value instead would build
	// LIKE '%   %' and return near-nothing.
	name := strings.TrimSpace(c.QueryParam("name"))

	// Tolerant parse: an absent or malformed flag means "active only".
	includeArchived, _ := strconv.ParseBool(c.QueryParam("include_archived"))

	return &entity.ListChannelsCmd{
		Name:            name,
		Limit:           paging.Limit,
		Offset:          paging.Offset,
		IncludeArchived: includeArchived,
	}
}

// channelUserIDs collects the non-nil author and editor ids across the channels.
func channelUserIDs(channels []*entity.NotifyChannel) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(channels)*2)
	for _, ch := range channels {
		if ch.CreatedByUserID != nil {
			ids = append(ids, *ch.CreatedByUserID)
		}
		if ch.UpdatedByUserID != nil {
			ids = append(ids, *ch.UpdatedByUserID)
		}
	}
	return ids
}

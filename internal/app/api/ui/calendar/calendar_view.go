package uicalendar

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	uimodels "github.com/ruko1202/maintmode/internal/app/api/ui/calendar/models"

	"github.com/ruko1202/maintmode/internal/utils/xtime"

	"github.com/ruko1202/maintmode/internal/apperr"

	"github.com/ruko1202/maintmode/internal/calendardto"
)

// CalendarView godoc
// @Summary List calendar events
// @Description Returns maintenance events for the specified date range.
// @Tags UI
// @Produce json
// @Param from query string true "Start date" Format(date)
// @Param to query string true "End date" Format(date)
// @Param statuses query []string false "Maintenance statuses" collectionFormat(multi) Enums(draft,planned,in_progress,canceled,completed)
// @Param resource_ids query []string false "Resource IDs(uuid)" collectionFormat(multi)
// @Success 200 {object} uimodels.CalendarViewResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid request"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 503 {object} httperrors.ErrorResponse "Auth service unavailable"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /ui/v1/calendar [get]
func (i *Implementation) CalendarView(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Calendar.CalendarView")
	defer span.End()
	op := "list calendar events"

	req := new(uimodels.CalendarViewRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseQuery)
	}

	req.To.Time = xtime.EndOfTheDay(req.To.Time)
	req.From.Time = xtime.StartOfTheDay(req.From.Time)

	if err := validateListEventsRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	maints, maintsMeta, err := i.calendarSrv.GetMaints(ctx, &calendardto.GetMaintsFilter{
		PeriodFrom:  req.From.Time,
		PeriodTo:    req.To.Time,
		Statuses:    req.Statuses,
		ResourceIDs: req.ResourceIDs,
	})
	if err != nil {
		xlog.Error(ctx, "get maintenances failed", xfield.Error(err))
		if errors.Is(err, apperr.ErrInvalidPeriodInterval) {
			return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
		}

		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, &uimodels.CalendarViewResponse{
		Meta: &uimodels.CalendarViewMeta{
			Truncated: maintsMeta.Truncated,
			Count:     maintsMeta.Count,
		},
		Events: lo.Map(maints, func(item *calendardto.Maintenance, _ int) *uimodels.CalendarEvent {
			return uimodels.ToAPICalendarEvent(item)
		}),
	})
}

func validateListEventsRequest(ctx context.Context, req *uimodels.CalendarViewRequest) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.From, validation.Required),
		validation.Field(&req.To, validation.Required,
			validation.WithContext(func(_ context.Context, _ interface{}) error {
				if req.To.Before(req.From.Time) {
					return fmt.Errorf("`to` date must be equal or after `from` date")
				}

				return nil
			}),
		),
		validation.Field(&req.Statuses, validation.Each(validation.Required)),
		validation.Field(&req.ResourceIDs, validation.Each(validation.Required)),
	)
}

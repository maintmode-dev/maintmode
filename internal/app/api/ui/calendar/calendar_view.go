package uicalendar

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
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
// @Param statuses query []uimodels.MaintenanceStatus false "Maintenance statuses" collectionFormat(multi)
// @Param resource_ids query []string false "Resource IDs(uuid)" collectionFormat(multi)
// @Success 200 {object} uimodels.CalendarViewResponse
// @Failure 400 {object} apierrors.ErrorResponse "Invalid request"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /ui/v1/calendar [get]
func (i *Implementation) CalendarView(c echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Calendar.CalendarView")
	defer span.End()

	req := new(uimodels.CalendarViewRequest)
	if err := c.Bind(req); err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			"cannot parse request body",
		))
	}
	req.To.Time = xtime.EndOfTheDay(req.To.Time)
	req.From.Time = xtime.StartOfTheDay(req.From.Time)

	if err := validateListEventsRequest(ctx, req); err != nil {
		xlog.Error(ctx, "invalid request", xfield.Error(err))
		return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
			apierrors.ErrInvalidRequest,
			err.Error(),
		))
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
			return c.JSON(http.StatusBadRequest, apierrors.NewErrorResponse(
				apierrors.ErrInvalidRequest,
				err.Error(),
			))
		}

		return c.JSON(http.StatusInternalServerError, apierrors.NewErrorResponse(
			apierrors.ErrInternalError,
			"get maintenances failed",
		))
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

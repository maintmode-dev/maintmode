package users

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/users/models"
)

// GetSeats godoc
// @Summary Get licensed seat usage
// @Description Returns how many licensed seats are taken and how many were purchased, so an admin can see the limit before an invite runs into it. Requires admin privileges.
// @Description seats_occupied (seats_used + seats_pending) is the number the seat cap is checked against: once it reaches seats_purchased, inviting into a seat role is rejected with seats_limit_exceeded.
// @Description A pending invitation to a seat role holds a seat from the moment it is sent; revoking or expiring it releases the seat. Guests never hold a seat.
// @Description unlimited is true on a self-hosted instance or before a ceiling has been reported, and seats_purchased is then null. Before a ceiling is reported the counters are still real; on a self-hosted instance no seats are tracked at all and every counter is zero, so render "unlimited" rather than "0 of unlimited". seats_occupied may exceed seats_purchased and is reported as is.
// @Tags Users
// @Produce json
// @Success 200 {object} apimodels.SeatsResponse
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/license/seats [get]
func (i *Implementation) GetSeats(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Users.GetSeats")
	defer span.End()
	op := "get seats"

	usage, license, err := i.licenseSrv.SeatsUsage(ctx)
	if err != nil {
		xlog.Error(ctx, "get seats usage failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// No ceiling known means no cap is enforced — the same predicate the seat
	// guard reads, so the indicator and the guard can never disagree.
	unlimited := license == nil || license.SeatsPurchased == nil

	resp := &apimodels.SeatsResponse{
		SeatsUsed:     usage.SeatsUsed(),
		SeatsPending:  usage.SeatsPending(),
		SeatsOccupied: usage.SeatsOccupied(),
		Unlimited:     unlimited,
	}
	if !unlimited {
		resp.SeatsPurchased = license.SeatsPurchased
	}

	return c.JSON(http.StatusOK, resp)
}

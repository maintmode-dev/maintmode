package invitations

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apimodels "github.com/ruko1202/maintmode/internal/app/api/public/invitations/models"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// ListInvitations godoc
// @Summary List invitations
// @Description Returns invitations (admin-view, may include sensitive fields), optionally filtered by status. Requires admin privileges.
// @Tags Users
// @Produce json
// @Param status query string false "Filter by status (pending|expired|accepted|revoked)"
// @Success 200 {object} apimodels.ListInvitationsResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid status filter"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/users/invitations [get]
func (i *Implementation) ListInvitations(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Invitations.ListInvitations")
	defer span.End()
	op := "list invitations"

	status, ok := entity.ParseInvitationStatusFilter(c.QueryParam("status"))
	if !ok {
		err := fmt.Errorf("unknown status filter %q", c.QueryParam("status"))
		xlog.Error(ctx, "invalid status filter", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	items, err := i.invSrv.List(ctx, &entity.ListInvitationsCmd{Status: status})
	if err != nil {
		xlog.Error(ctx, "list invitations failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, &apimodels.ListInvitationsResponse{
		Invitations: apimodels.ToAPIInvitations(items, xtime.UTCNow()),
	})
}

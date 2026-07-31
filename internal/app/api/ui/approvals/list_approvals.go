package uiapprovals

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	approvalsmodels "github.com/ruko1202/maintmode/internal/app/api/ui/approvals/models"
	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/calendardto"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

const (
	defaultApprovalsLimit  = 50
	maxApprovalsLimit      = 200
	defaultApprovalsOffset = 0
)

// ListApprovals godoc
// @Summary List maintenances awaiting my approval
// @Description Returns draft maintenances where the caller is the assigned approver,
// @Description oldest first. The approver is taken from the access token — there is no
// @Description parameter to request another user's queue.
// @Description Malformed pagination params are coerced to defaults rather than rejected.
// @Tags UI
// @Produce json
// @Param limit query int false "Page size; values <= 0 or above 200 fall back to 50, not to 200" default(50)
// @Param offset query int false "Pagination offset; negative values fall back to 0" default(0)
// @Success 200 {object} approvalsmodels.ListApprovalsResponse
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /ui/v1/approvals [get]
func (i *Implementation) ListApprovals(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Approvals.ListApprovals")
	defer span.End()
	op := "list pending approvals"

	// The queue is personal by construction: the approver comes from the token,
	// and a missing user is a hard failure rather than a system-actor fallback.
	user, ok := xecho.UserFromEchoCtx(c)
	if !ok {
		return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
	}

	filter := queryToPendingApprovalsFilter(c, user.ID)

	result, err := i.calendarSrv.ListPendingApprovals(ctx, filter)
	if err != nil {
		xlog.Error(ctx, "list pending approvals failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	// Batch-resolve every author across the page in one auth call (degrades to
	// "Unknown user" on failure, never errors the read).
	summaries := i.userSummarySrv.ResolveMany(ctx, approvalsmodels.ApprovalUserIDs(result.Maintenances))

	return c.JSON(http.StatusOK, &approvalsmodels.ListApprovalsResponse{
		Maintenances: approvalsmodels.ToAPIApprovalRows(result.Maintenances, summaries),
		Total:        result.Total,
		Limit:        filter.Limit,
		Offset:       filter.Offset,
	})
}

// queryToPendingApprovalsFilter parses pagination query params. Malformed or
// out-of-range values are silently coerced to safe defaults rather than
// rejected — this is a read-only list, so a best-effort response beats a 400.
func queryToPendingApprovalsFilter(c *echo.Context, approverID uuid.UUID) *calendardto.PendingApprovalsFilter {
	limit, err := echo.QueryParamOr[int64](c, "limit", defaultApprovalsLimit)
	if err != nil || limit <= 0 || limit > maxApprovalsLimit {
		limit = defaultApprovalsLimit
	}

	offset, err := echo.QueryParamOr[int64](c, "offset", defaultApprovalsOffset)
	if err != nil || offset < 0 {
		offset = defaultApprovalsOffset
	}

	return &calendardto.PendingApprovalsFilter{
		ApproverUserID: approverID,
		Limit:          limit,
		Offset:         offset,
	}
}

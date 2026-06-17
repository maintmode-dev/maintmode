package audit

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xlo"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/audit/models"
	"github.com/ruko1202/maintmode/internal/entity"
)

const (
	defaultMaxLogsCount = 100
	defaultOffset       = 0
)

// AuditLog godoc
// @Summary Get audit log
// @Description Returns one page of audit log entries ordered by created_at DESC,
// @Description plus the total count under the current filter and facet counts per
// @Description category (computed in the actor/date window, without the action filter).
// @Description Supports optional filtering by action, actor, and created_at range, plus offset/limit pagination.
// @Description Each entry carries actor_id / actor_display_name (write-time snapshot), a stable target_id,
// @Description and a structured action-specific metadata object (see AuditLogMetadata for which fields are set per action).
// @Tags Audit
// @Produce json
// @Param limit query int false "Number of entries to return (max 100)" default(100)
// @Param offset query int false "Pagination offset" default(0)
// @Param action query string false "Filter by audit actions. CSV of one or more of the listed values (e.g. login.success,maintenance.canceled)" Enums(login.success, login.failed, logout.success, roles.changed, user.blocked, user.unblocked, maintenance.created, maintenance.updated, maintenance.approved, maintenance.started, maintenance.completed, maintenance.canceled, maintenance_step.started, maintenance_step.completed, maintenance_step.canceled)
// @Param actor query string false "Filter by actor (exact match)"
// @Param created_from query string false "Filter by created_at >= this RFC3339 timestamp"
// @Param created_to query string false "Filter by created_at <= this RFC3339 timestamp"
// @Success 200 {object} apiauthmodels.AuditLogResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid query parameters"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/audit/log [get]
func (i *Implementation) AuditLog(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Audit.AuditLog")
	defer span.End()
	op := "audit log"

	cmd, err := queryToGetAuditLogsCmd(ctx, c)
	if err != nil {
		xlog.Error(ctx, "query to audit log command failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ValidationErr(err))
	}

	page, err := i.auditSrv.GetLogs(ctx, cmd)
	if err != nil {
		xlog.Error(ctx, "get audit log failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPIAuditLogResponse(page))
}

func queryToGetAuditLogsCmd(ctx context.Context, c *echo.Context) (*entity.GetAuditLogsCmd, error) {
	limit, err := echo.QueryParamOr[int64](c, "limit", defaultMaxLogsCount)
	if err != nil {
		xlog.Error(ctx, "parse limit failed", xfield.Error(err))
		return nil, fmt.Errorf("invalid limit")
	}
	if limit <= 0 || limit > defaultMaxLogsCount {
		limit = defaultMaxLogsCount
	}

	offset, err := echo.QueryParamOr[int64](c, "offset", defaultOffset)
	if err != nil {
		xlog.Error(ctx, "parse offset failed", xfield.Error(err))
		return nil, fmt.Errorf("invalid offset")
	}
	if offset < 0 {
		offset = defaultOffset
	}

	from, err := parseTimeQuery(c, "created_from")
	if err != nil {
		xlog.Error(ctx, "parse created_from failed", xfield.Error(err))
		return nil, fmt.Errorf("invalid created_from")
	}

	to, err := parseTimeQuery(c, "created_to")
	if err != nil {
		xlog.Error(ctx, "parse created_to failed", xfield.Error(err))
		return nil, fmt.Errorf("invalid created_to")
	}

	actions, err := parseActionsQuery(c.QueryParam("action"))
	if err != nil {
		xlog.Error(ctx, "parse action failed", xfield.Error(err))
		return nil, err
	}

	return &entity.GetAuditLogsCmd{
		Limit:  limit,
		Offset: offset,
		Filter: &entity.AuditFilter{
			CreatedFrom: from,
			CreatedTo:   to,
			Actions:     actions,
			Actor:       xlo.ToPtrOrNil(c.QueryParam("actor")),
		},
	}, nil
}

// parseActionsQuery parses the "action" query param as a CSV of audit
// actions, so one FE category chip can expand into several enum values.
func parseActionsQuery(actionsQuery string) ([]entity.AuditAction, error) {
	if actionsQuery == "" {
		return nil, nil
	}

	parts := strings.Split(actionsQuery, ",")
	actions := make([]entity.AuditAction, 0, len(parts))
	for _, part := range parts {
		action := entity.AuditAction(strings.TrimSpace(part))
		if !action.IsValid() {
			return nil, fmt.Errorf("invalid action %q", action)
		}
		actions = append(actions, action)
	}

	return actions, nil
}

func parseTimeQuery(c *echo.Context, name string) (*time.Time, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("must be RFC3339: %w", err)
	}
	return &t, nil
}

package audit

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	apiauthmodels "github.com/ruko1202/maintmode/internal/app/api/public/audit/models"
)

const defaultMaxLogsCount = 100

// AuditLog godoc
// @Summary Get audit log
// @Description Returns the last N audit log entries (default and max: 100).
// @Tags Audit
// @Produce json
// @Param limit query int false "Number of entries to return (max 100)" default(100)
// @Success 200 {object} apiauthmodels.AuditLogResponse
// @Failure 400 {object} httperrors.ErrorResponse "Invalid limit parameter"
// @Failure 401 {object} httperrors.ErrorResponse "Unauthorized"
// @Failure 403 {object} httperrors.ErrorResponse "Forbidden"
// @Failure 500 {object} httperrors.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/audit/log [get]
func (i *Implementation) AuditLog(c *echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Audit.AuditLog")
	defer span.End()
	op := "audit log"

	limit, err := strconv.ParseInt(c.QueryParamOr("limit", fmt.Sprint(defaultMaxLogsCount)), 10, 64)
	if err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, httperrors.ErrParseBody)
	}

	if limit <= 0 || limit > defaultMaxLogsCount {
		limit = defaultMaxLogsCount
	}

	logs, err := i.auditSrv.GetLogs(ctx, limit)
	if err != nil {
		xlog.Error(ctx, "get audit log failed", xfield.Error(err))
		return httperrors.ToAPIError(c, op, err)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPIAuditLogResponse(logs))
}

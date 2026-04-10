package audit

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/app/api/apierrors"
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
// @Failure 400 {object} apierrors.ErrorResponse "Invalid limit parameter"
// @Failure 500 {object} apierrors.ErrorResponse "Internal error"
// @Router /api/v1/audit/log [get]
func (i *Implementation) AuditLog(c echo.Context) error {
	ctx, span := xlog.WithOperationSpan(c.Request().Context(), "api.Audit.AuditLog")
	defer span.End()
	op := "audit log"

	limit, err := strconv.ParseInt(c.QueryParam("limit"), 10, 64)
	if err != nil {
		xlog.Error(ctx, "bind request failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, apierrors.ErrParseBody)
		return c.JSON(statusCode, errResp)
	}

	if limit == 0 || limit > defaultMaxLogsCount {
		limit = defaultMaxLogsCount
	}

	logs, err := i.auditSrv.GetLogs(ctx, limit)
	if err != nil {
		xlog.Error(ctx, "get audit log failed", xfield.Error(err))
		statusCode, errResp := apierrors.ToAPIErrResponse(op, err)
		return c.JSON(statusCode, errResp)
	}

	return c.JSON(http.StatusOK, apiauthmodels.ToAPIAuditLogResponse(logs))
}

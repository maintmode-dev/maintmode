package healthcheck

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"
)

// Readiness checks database connectivity and returns HTTP 200 OK if ready, 500 if not.
func (a *Implementation) Readiness(c echo.Context) error {
	ctx := xlog.WithOperation(c.Request().Context(), "status.Readiness")

	err := a.db.PingContext(ctx)
	if err != nil {
		xlog.Error(ctx, "db ping failed", zap.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}

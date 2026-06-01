package infra

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

func (i *Implementation) Readiness(c *echo.Context) error {
	ctx := xlog.WithOperation(c.Request().Context(), "status.Readiness")

	// During graceful shutdown report not-ready so the load balancer drains
	// this replica before the server stops accepting connections. This is
	// what makes rolling deploys zero-downtime: the proxy ejects us from the
	// pool while we keep serving in-flight requests.
	if i.drain.IsDraining() {
		return c.NoContent(http.StatusServiceUnavailable)
	}

	err := i.db.PingContext(ctx)
	if err != nil {
		xlog.Error(ctx, "db ping failed", xfield.Error(err))
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusNoContent)
}

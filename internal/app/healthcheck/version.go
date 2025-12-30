package healthcheck

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

// Version returns the application build metadata as JSON.
func (a *Implementation) Version(c echo.Context) error {
	return c.JSON(http.StatusOK, buildmeta.GetAppBuildMeta())
}

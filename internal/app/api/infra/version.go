package infra

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"
)

func (i *Implementation) Version(c echo.Context) error {
	return c.JSON(http.StatusOK, buildmeta.GetAppBuildMeta())
}

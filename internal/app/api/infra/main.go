package infra

import (
	"net/http"

	"github.com/labstack/echo/v4"

	webinfra "github.com/ruko1202/maintmode/web/static/infra"
)

func (i *Implementation) MainPage(c echo.Context) error {
	return c.HTMLBlob(http.StatusOK, webinfra.MainPage)
}

package infra

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (i *Implementation) Liveness(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

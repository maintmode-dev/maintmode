package healthcheck

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Liveness returns HTTP 200 OK indicating the application is alive and running.
func (a *Implementation) Liveness(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

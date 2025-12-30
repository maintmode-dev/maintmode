// Package healthcheck provides HTTP handlers for health check endpoints.
// It includes liveness, readiness, and version endpoints for monitoring application health.
package healthcheck

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// Implementation provides health check handlers with database connectivity.
type Implementation struct {
	db *sqlx.DB
}

// NewImplementation creates a new health check implementation with the provided database connection.
func NewImplementation(db *sqlx.DB) *Implementation {
	return &Implementation{
		db: db,
	}
}

// BindRoute registers health check routes (liveness, readiness, version) to the provided Echo group.
func (a *Implementation) BindRoute(gr *echo.Group, middlewares []echo.MiddlewareFunc) {
	for route, handler := range map[*echo.Route]struct {
		handler    echo.HandlerFunc
		middleware []echo.MiddlewareFunc
	}{
		{Path: "/liveness", Name: "Liveness", Method: http.MethodGet}: {
			handler: a.Liveness,
		},
		{Path: "/readiness", Name: "Readiness", Method: http.MethodGet}: {
			handler: a.Readiness,
		},
		{Path: "/version", Name: "Version", Method: http.MethodGet}: {
			handler: a.Version,
		},
	} {
		gr.Add(route.Method, route.Path, handler.handler, handler.middleware...)
	}

	gr.Use(middlewares...)
}

// Package api provides the main application handlers for the MaintMode dashboard.
// It manages maintenance work schedules through a calendar interface.
package api

import (
	"github.com/labstack/echo/v4"
)

// Implementation provides the main application route handlers.
type Implementation struct {
}

// New creates a new maintmode implementation.
func New() *Implementation {
	return &Implementation{}
}

// BindRoute registers maintmode application routes to the provided Echo group.
func (a *Implementation) BindRoute(gr *echo.Group, middlewares []echo.MiddlewareFunc) {
	for route, handler := range map[*echo.Route]struct {
		handler    echo.HandlerFunc
		middleware []echo.MiddlewareFunc
	}{} {
		gr.Add(route.Method, route.Path, handler.handler, handler.middleware...)
	}

	gr.Use(middlewares...)
}

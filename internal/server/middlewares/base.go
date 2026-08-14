// Package middlewares provides Echo HTTP middleware configurations.
// It includes base security, recovery, request ID, and logging middlewares.
package middlewares

import (
	"time"

	echootel "github.com/labstack/echo-opentelemetry"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	xhttpserver "github.com/ruko1202/xhttp/server"
	"go.opentelemetry.io/otel/trace"

	"github.com/ruko1202/maintmode/internal/config/buildmeta"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// BaseAPIMiddlewares returns the basic set of middlewares (recover, secure, request ID) for public API
//
// TraceMiddleware must stay ahead of xhttpserver.RequestLoggingMiddleware: it
// overwrites X-Request-ID with the trace id, and the logging middleware reads
// that header afterwards. Reordering the two silently decouples the logs from
// the traces — the request_id field keeps a value, just no longer the one that
// correlates.
func BaseAPIMiddlewares(env config.Environment, meta *buildmeta.AppBuildMeta) []echo.MiddlewareFunc {
	mw := append(xhttpserver.BaseMiddlewares(),
		middleware.Recover(),
		middleware.Secure(),
		echootel.NewMiddleware(meta.AppName),
		middleware.RequestIDWithConfig(middleware.RequestIDConfig{Generator: xuuid.NewString}),
		TraceMiddleware(),
		xhttpserver.RequestLoggingMiddleware(),
		middleware.ContextTimeout(60*time.Second),
		middleware.GzipWithConfig(middleware.GzipConfig{}),
	)

	if env.IsDev() {
		mw = append(mw,
			middleware.CORS("*"),
			// Dev-only: honor X-Test-Roles so tests/k6 can request roles at login.
			// Gated here (not in prod) so the header is physically unread in prod.
			// Kept in the shared IsDev branch — performance_test (k6) needs it too.
			InjectTestRoles(),
		)
		if !env.IsPerformanceTest() {
			mw = append(mw,
				xhttpserver.BodyDumpLoggingMiddleware(),
			)
		}
	}

	return mw
}

// TraceMiddleware Middleware for injecting TraceID into X-Request-ID response header
func TraceMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()
			spanCtx := trace.SpanContextFromContext(ctx)

			// If trace was successfully created/received
			if spanCtx.HasTraceID() {
				traceID := spanCtx.TraceID().String()

				// Return to client in response header
				c.Response().Header().Set(echo.HeaderXRequestID, traceID)

				// Store in Echo context (so Echo's standard loggers can pick it up)
				c.Set(echo.HeaderXRequestID, traceID)
			}
			return next(c)
		}
	}
}

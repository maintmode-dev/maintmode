// Package middlewares provides Echo HTTP middleware configurations.
// It includes base security, recovery, request ID, and logging middlewares.
package middlewares

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// BaseMiddlewares returns the basic set of middlewares (recover, secure, request ID).
func BaseMiddlewares() []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{
		middleware.Recover(),
		middleware.Secure(),
		middleware.RequestIDWithConfig(middleware.RequestIDConfig{Generator: xuuid.NewString}),
	}
}

// BaseWithLoggingMiddlewares returns base middlewares plus structured request logging.
func BaseWithLoggingMiddlewares(app string) []echo.MiddlewareFunc {
	ctx := xlog.WithOperation(context.Background(), app)
	return append(BaseMiddlewares(),
		middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			LogLatency:       true,
			LogProtocol:      false,
			LogRemoteIP:      true,
			LogHost:          true,
			LogMethod:        true,
			LogURI:           true,
			LogURIPath:       false,
			LogRoutePath:     false,
			LogRequestID:     true,
			LogReferer:       false,
			LogUserAgent:     true,
			LogStatus:        true,
			LogError:         true,
			LogContentLength: true,
			LogResponseSize:  true,
			LogHeaders:       nil,
			LogQueryParams:   nil,
			LogFormValues:    nil,
			HandleError:      true, // forwards error to the global error handler, so it can decide appropriate status code
			LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
				attrs := []zap.Field{
					zap.String("method", v.Method),
					zap.String("uri", v.URI),
					zap.Int("status", v.Status),
					zap.Duration("latency", v.Latency),
					zap.String("host", v.Host),
					zap.String("bytes_in", v.ContentLength),
					zap.Int64("bytes_out", v.ResponseSize),
					zap.String("user_agent", v.UserAgent),
					zap.String("remote_ip", v.RemoteIP),
					zap.String("request_id", v.RequestID),
				}
				if v.Error != nil {
					xlog.Error(ctx, "REQUEST_ERROR", append(attrs, zap.Error(v.Error))...)
					return nil //nolint:nilerr
				}

				xlog.Info(ctx, "REQUEST", attrs...)
				return nil
			},
		}),
	)
}

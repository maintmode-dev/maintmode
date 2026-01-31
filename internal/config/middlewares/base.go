// Package middlewares provides Echo HTTP middleware configurations.
// It includes base security, recovery, request ID, and logging middlewares.
package middlewares

import (
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/ruko1202/xlog"
	"go.uber.org/zap"

	"github.com/ruko1202/maintmode/internal/config"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

const (
	msxResponseDump = 10 * 1024 // 10Kb
)

// BaseMiddlewares returns the basic set of middlewares (recover, secure, request ID).
func BaseMiddlewares() []echo.MiddlewareFunc {
	mw := []echo.MiddlewareFunc{
		middleware.Recover(),
		middleware.Secure(),
		middleware.RequestIDWithConfig(middleware.RequestIDConfig{Generator: xuuid.NewString}),
		middleware.GzipWithConfig(middleware.GzipConfig{
			Skipper: skipper("swagger"),
		}),
	}

	if config.GetAppConfig().IsDevEnvironment() {
		mw = append(mw,
			middleware.CORS(),
		)
	}

	return mw
}

func RequestLoggingMiddleware() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogLatency:       true,
		LogProtocol:      true,
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
		Skipper:          skipper("swagger"),
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			ctx := c.Request().Context()
			attrs := []zap.Field{
				zap.String("request", fmt.Sprintf("%s %s", v.Method, v.URI)),
				zap.String("protocol", v.Protocol),
				zap.Int("status", v.Status),
				zap.String("request_id", v.RequestID),
				zap.Any("query", v.QueryParams),
				zap.Any("form-values", v.FormValues),
				zap.Duration("latency", v.Latency),
				zap.String("bytes_in", v.ContentLength),
				zap.Int64("bytes_out", v.ResponseSize),
				zap.String("remote_ip", v.RemoteIP),
				zap.String("user_agent", v.UserAgent),
				zap.Any("headers", v.Headers),
			}

			if v.Error != nil {
				xlog.Error(ctx, "REQUEST_ERROR", append(attrs, zap.Error(v.Error))...)
				return nil //nolint:nilerr
			}

			xlog.Info(ctx, "REQUEST", attrs...)
			return nil
		},
	})
}

func ReqReqsDumpLoggingMiddleware() echo.MiddlewareFunc {
	return middleware.BodyDumpWithConfig(middleware.BodyDumpConfig{
		Skipper: skipper("swagger"),
		Handler: func(c echo.Context, reqDump []byte, respBump []byte) {
			req := c.Request()
			res := c.Response()
			ctx := req.Context()

			requestID := req.Header.Get(echo.HeaderXRequestID)
			if requestID == "" {
				requestID = res.Header().Get(echo.HeaderXRequestID)
			}

			attrs := []zap.Field{
				zap.String("method", req.Method),
				zap.String("uri", req.RequestURI),
				zap.String("request_id", requestID),
			}

			xlog.Info(ctx, "REQUEST DUMP", append(attrs, zap.ByteString("body", reqDump))...)
			if len(respBump) > msxResponseDump {
				xlog.Info(ctx, fmt.Sprintf("RESPONSE DUMP skipped: too large response body [%d Kb]", len(respBump)/1024), attrs...)
				return
			}
			xlog.Info(ctx, "RESPONSE DUMP", append(attrs, zap.ByteString("body", respBump))...)
		},
	})
}

func skipper(urlPaths ...string) middleware.Skipper {
	return func(c echo.Context) bool {
		for _, path := range urlPaths {
			if strings.Contains(c.Request().URL.Path, path) {
				return true
			}
		}
		return false
	}
}

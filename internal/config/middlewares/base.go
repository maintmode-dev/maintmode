// Package middlewares provides Echo HTTP middleware configurations.
// It includes base security, recovery, request ID, and logging middlewares.
package middlewares

import (
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

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
			attrs := []xfield.Field{
				xfield.String("request", fmt.Sprintf("%s %s", v.Method, v.URI)),
				xfield.String("protocol", v.Protocol),
				xfield.Int("status", v.Status),
				xfield.String("request_id", v.RequestID),
				xfield.Any("query", v.QueryParams),
				xfield.Any("form-values", v.FormValues),
				xfield.Duration("latency", v.Latency),
				xfield.String("bytes_in", v.ContentLength),
				xfield.Int64("bytes_out", v.ResponseSize),
				xfield.String("remote_ip", v.RemoteIP),
				xfield.String("user_agent", v.UserAgent),
				xfield.Any("headers", v.Headers),
			}

			if v.Error != nil {
				xlog.Error(ctx, "REQUEST_ERROR", append(attrs, xfield.Error(v.Error))...)
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

			attrs := []xfield.Field{
				xfield.String("method", req.Method),
				xfield.String("uri", req.RequestURI),
				xfield.String("request_id", requestID),
			}

			xlog.Info(ctx, "REQUEST DUMP", append(attrs, xfield.String("body", string(reqDump)))...)
			if len(respBump) > msxResponseDump {
				xlog.Info(ctx, fmt.Sprintf("RESPONSE DUMP skipped: too large response body [%d Kb]", len(respBump)/1024), attrs...)
				return
			}
			xlog.Info(ctx, "RESPONSE DUMP", append(attrs, xfield.String("body", string(respBump)))...)
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

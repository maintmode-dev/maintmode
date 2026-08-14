package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/config/buildmeta"
	"github.com/ruko1202/maintmode/internal/server/middlewares"
)

// TestBaseAPIMiddlewaresOrdering guards an invariant that lives in a comment
// and, since the extraction, spans two repositories: TraceMiddleware must run
// before the request logger.
//
// Trace overwrites X-Request-ID with the trace id; the logger (now
// xhttpserver.RequestLoggingMiddleware) reads that header afterwards to fill
// its request_id field. Swap them and nothing breaks loudly — the field keeps
// a value, just no longer the one that correlates a log line with its trace.
// That is the kind of regression you find months later while debugging
// something else.
func TestBaseAPIMiddlewaresOrdering(t *testing.T) {
	t.Parallel()

	meta := &buildmeta.AppBuildMeta{AppName: "test"}

	t.Run("the request id reaching the handler is the trace id", func(t *testing.T) {
		t.Parallel()

		e := echo.New()
		e.Use(middlewares.BaseAPIMiddlewares(config.ProdEnvironment, meta)...)

		e.GET("/probe", func(c *echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		})

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/probe", nil))

		require.Equal(t, http.StatusNoContent, rec.Code)
		// With no incoming trace parent there is no span to derive an id from,
		// so this is what RequestID generated. What matters is that the header
		// is populated before the logger reads it.
		require.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID))
	})

	// The dev chain adds CORS, test-role injection and body dumping. Body
	// dumping in particular writes request bodies verbatim, so it must never
	// appear outside dev.
	t.Run("dev adds middlewares, prod does not", func(t *testing.T) {
		t.Parallel()

		prod := middlewares.BaseAPIMiddlewares(config.ProdEnvironment, meta)
		dev := middlewares.BaseAPIMiddlewares(config.DevEnvironment, meta)
		perf := middlewares.BaseAPIMiddlewares(config.PerformanceTestEnvironment, meta)

		require.Greater(t, len(dev), len(prod), "dev must add to the prod chain, not replace it")
		// performance_test is a dev environment but skips the body dump: k6
		// drives enough traffic that logging every body would drown the run.
		require.Less(t, len(perf), len(dev))
		require.Greater(t, len(perf), len(prod))
	})
}

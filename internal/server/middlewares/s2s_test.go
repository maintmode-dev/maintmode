package middlewares

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/ruko1202/maintmode/internal/config"
)

const (
	s2sTestApp    = "maintmode-ui"
	s2sTestSecret = "0123456789abcdef0123456789abcdef"
	s2sTestPath   = "/api/v1/refresh"
)

func TestRequireS2SToken(t *testing.T) {
	t.Parallel()

	cfg := config.S2SConfig{
		s2sTestApp: {Secret: s2sTestSecret, AvailableAPI: []string{s2sTestPath}},
	}

	cases := []struct {
		name    string
		path    string
		appName string
		token   string
		pass    bool
	}{
		{name: "the configured pair on an allowed path", path: s2sTestPath, appName: s2sTestApp, token: s2sTestSecret, pass: true},
		{name: "no headers", path: s2sTestPath},
		{name: "no token", path: s2sTestPath, appName: s2sTestApp},
		{name: "no appname", path: s2sTestPath, token: s2sTestSecret},
		{name: "unknown appname", path: s2sTestPath, appName: "someone-else", token: s2sTestSecret},
		{name: "wrong token", path: s2sTestPath, appName: s2sTestApp, token: strings.Repeat("x", len(s2sTestSecret))},
		{name: "a prefix of the token", path: s2sTestPath, appName: s2sTestApp, token: s2sTestSecret[:16]},
		{name: "a path the client is not allowed", path: "/api/v1/login/password", appName: s2sTestApp, token: s2sTestSecret},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called, rec := serveS2S(context.Background(), t, cfg, tc.path, tc.appName, tc.token)

			if tc.pass {
				require.True(t, called)
				require.Equal(t, http.StatusNoContent, rec.Code)
				return
			}

			require.False(t, called, "a rejected client must never reach the handler")
			require.Equal(t, http.StatusUnauthorized, rec.Code)
			require.JSONEq(t, `{"code":"invalid_client","message":"client authentication failed"}`, rec.Body.String(),
				"every reason answers the same body")
		})
	}
}

// The rejection log is the operator's only clue to WHY a client was refused —
// the response says nothing — but it must never carry a credential. An appname
// that names no configured client is logged only as its length and a short
// digest: a client that swaps its two headers sends its 64-character secret as
// the appname.
func TestRequireS2SToken_RejectionLog(t *testing.T) {
	t.Parallel()

	cfg := config.S2SConfig{s2sTestApp: {Secret: s2sTestSecret, AvailableAPI: []string{s2sTestPath}}}

	logLine := func(t *testing.T, appName, token string) string {
		t.Helper()

		core, logs := observer.New(zapcore.WarnLevel)
		ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zap.New(core)))
		serveS2S(ctx, t, cfg, s2sTestPath, appName, token)

		entries := logs.All()
		require.Len(t, entries, 1)
		raw, err := json.Marshal(entries[0].ContextMap())
		require.NoError(t, err)

		return entries[0].Message + string(raw)
	}

	t.Run("swapped headers: the secret sent as the appname is not logged", func(t *testing.T) {
		t.Parallel()

		secretAsAppName := strings.Repeat("ab", 32)
		line := logLine(t, secretAsAppName, s2sTestApp)

		require.NotContains(t, line, secretAsAppName[:16])
		require.Contains(t, line, `"appname_len":64`)
	})

	t.Run("a configured appname is logged as is, the token never", func(t *testing.T) {
		t.Parallel()

		line := logLine(t, s2sTestApp, strings.Repeat("x", len(s2sTestSecret)))

		require.Contains(t, line, `"appname":"`+s2sTestApp+`"`)
		require.NotContains(t, line, strings.Repeat("x", 8))
		require.NotContains(t, line, s2sTestSecret)
	})
}

// With more than one caller configured, each is held to its own secret and its
// own routes: a credential or an allowance of one client must not open a route
// for another.
func TestRequireS2SToken_ClientsAreIsolated(t *testing.T) {
	t.Parallel()

	const (
		otherApp    = "another-service"
		otherSecret = "fedcba9876543210fedcba9876543210"
		otherPath   = "/api/v1/login/password"
	)
	cfg := config.S2SConfig{
		s2sTestApp: {Secret: s2sTestSecret, AvailableAPI: []string{s2sTestPath}},
		otherApp:   {Secret: otherSecret, AvailableAPI: []string{otherPath}},
	}

	cases := []struct {
		name    string
		path    string
		appName string
		token   string
	}{
		{name: "another client's secret under this appname", path: s2sTestPath, appName: s2sTestApp, token: otherSecret},
		{name: "a route only another client may use", path: otherPath, appName: s2sTestApp, token: s2sTestSecret},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called, rec := serveS2S(context.Background(), t, cfg, tc.path, tc.appName, tc.token)

			require.False(t, called, "a rejected client must never reach the handler")
			require.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

// serveS2S runs one POST to path through RequireS2SToken(cfg) with the given
// s2s headers ("" = header absent) and reports whether the handler was reached.
func serveS2S(ctx context.Context, t *testing.T, cfg config.S2SConfig, path, appName, token string) (bool, *httptest.ResponseRecorder) {
	t.Helper()

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, path, http.NoBody)
	if appName != "" {
		req.Header.Set(config.XS2STokenAppNameHeader, appName)
	}
	if token != "" {
		req.Header.Set(config.XS2STokenHeader, token)
	}
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)
	c.SetPath(path)

	called := false
	err := RequireS2SToken(cfg)(func(c *echo.Context) error {
		called = true
		return c.NoContent(http.StatusNoContent)
	})(c)
	require.NoError(t, err)

	return called, rec
}

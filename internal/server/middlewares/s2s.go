package middlewares

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/config"
)

type s2sClient struct {
	secret        []byte
	availableAPIs map[string]struct{}
}

// RequireS2SToken authenticates the calling service on routes that issue or
// redeem a token pair. The caller names itself in X-S2S-APPNAME and proves it
// with the shared secret in X-S2S-TOKEN; the route must be in that caller's
// available_api.
//
// It runs ahead of the handler, so a refused caller never reaches the refresh
// token: no lock, no lookup, no reuse detection, no family revocation.
//
// Every refusal answers the same 401 invalid_client body — which check failed
// is logged, never returned. Tokens are compared in constant time.
func RequireS2SToken(s2s config.S2SConfig) echo.MiddlewareFunc {
	clients := lo.MapValues(s2s, func(client config.S2S, _ string) s2sClient {
		return s2sClient{
			secret:        []byte(client.Secret),
			availableAPIs: lo.Keyify(client.AvailableAPI),
		}
	})

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := c.Request().Header
			appName := header.Get(config.XS2STokenAppNameHeader)
			token := header.Get(config.XS2STokenHeader)

			client, known := clients[appName]

			switch {
			case token == "" || appName == "":
				return rejectS2SClient(c, "missing s2s headers", appName, known)
			case !known:
				return rejectS2SClient(c, "unknown s2s appname", appName, known)
			}

			if _, ok := client.availableAPIs[c.Path()]; !ok {
				return rejectS2SClient(c, "route not allowed for s2s appname", appName, known)
			}

			if subtle.ConstantTimeCompare([]byte(token), client.secret) != 1 {
				return rejectS2SClient(c, "invalid s2s token", appName, known)
			}

			return next(c)
		}
	}
}

// rejectS2SClient answers the refusal and logs why. Only a configured appname
// is logged as is; anything else came from the caller and may be a credential —
// a client that swaps its two headers sends its secret as the appname — so it
// is logged as its length and a short digest, enough to tell callers apart.
func rejectS2SClient(c *echo.Context, reason, appName string, known bool) error {
	appField := xfield.String("appname", appName)
	if !known {
		digest := sha256.Sum256([]byte(appName))
		appField = xfield.String("appname_sha256", hex.EncodeToString(digest[:4]))
	}

	xlog.Warn(c.Request().Context(), "s2s client rejected",
		xfield.String("reason", reason),
		appField,
		xfield.Int("appname_len", len(appName)),
		xfield.String("route", c.Path()),
	)

	return c.JSON(http.StatusUnauthorized,
		httperrors.NewErrorResponse(httperrors.ErrInvalidClient, "client authentication failed"))
}

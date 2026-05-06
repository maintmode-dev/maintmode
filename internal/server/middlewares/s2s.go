package middlewares

import (
	"crypto/subtle"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/samber/lo"

	"github.com/ruko1202/maintmode/internal/config"
)

type s2sConfig struct {
	token         string
	availableAPIs map[string]struct{}
}

// RequireS2SToken returns middleware that validates a static service token
// from the Authorization header (Bearer <token>). It is designed for
// service-to-service endpoints like /auth/introspect where only trusted
// backend services should have access.
//
// Tokens are compared in constant time to prevent timing attacks.
func RequireS2SToken(s2s config.S2SConfig) echo.MiddlewareFunc {
	s2sCfg := lo.MapEntries(s2s, func(serviceName string, s2sCfg config.S2S) (string, s2sConfig) {
		return serviceName, s2sConfig{
			token: s2sCfg.Secret,
			availableAPIs: lo.SliceToMap(s2sCfg.AvailableAPI, func(item string) (string, struct{}) {
				return item, struct{}{}
			}),
		}
	})

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			reqHeader := c.Request().Header

			headerS2SToken := reqHeader.Get(config.XS2STokenHeader)
			if headerS2SToken == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing s2s token")
			}

			headerS2SAppName := reqHeader.Get(config.XS2STokenAppNameHeader)
			if headerS2SAppName == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing s2s appName")
			}

			serviceS2S, ok := s2sCfg[headerS2SAppName]
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid s2s appName")
			}

			if _, ok := serviceS2S.availableAPIs[c.Path()]; !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "access to the URI not allowed")
			}

			if subtle.ConstantTimeCompare([]byte(headerS2SToken), []byte(serviceS2S.token)) != 1 {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid s2s token")
			}

			return next(c)
		}
	}
}

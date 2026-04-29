package middlewares

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

type TokenVerifier interface {
	VerifyAccessToken(ctx context.Context, tokenString string) (*entity.AccessClaims, error)
}

// JWTAuth создаёт middleware для проверки JWT токенов.
// Парсит Authorization: Bearer <token>, верифицирует подпись,
// записывает userID
func JWTAuth(tokenSrv TokenVerifier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := req.Context()

			authHeader := req.Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing oauth token")
			}

			authToken, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid oauth token")
			}

			claims, err := tokenSrv.VerifyAccessToken(ctx, authToken)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid oauth token")
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid claims")
			}

			xecho.UserToEchoCtx(c, &entity.User{
				ID:    userID,
				Email: claims.Email,
				Roles: claims.Roles,
			})

			return next(c)
		}
	}
}

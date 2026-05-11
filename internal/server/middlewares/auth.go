package middlewares

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xhttp"

	"github.com/ruko1202/maintmode/internal/app/api/httperrors"
	"github.com/ruko1202/maintmode/internal/utils/xvalidation"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xecho"
)

type TokenVerifier interface {
	VerifyAccessToken(ctx context.Context, tokenString string) (*entity.AccessClaims, error)
}

// RequireAccessToken создаёт middleware для проверки JWT токенов.
// Парсит Authorization: Bearer <token>, верифицирует подпись,
// записывает userID
func RequireAccessToken(tokenSrv TokenVerifier) echo.MiddlewareFunc {
	op := "authenticate"

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := req.Context()

			authToken := xhttp.ExtractBearerToken(c.Request())
			if authToken == "" {
				return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
			}

			claims, err := tokenSrv.VerifyAccessToken(ctx, authToken)
			if err != nil {
				return httperrors.ToAPIError(c, op, err)
			}

			user, err := userFromAccessClaims(ctx, claims)
			if err != nil {
				xlog.Error(ctx, "invalid access token claims", xfield.Error(err))
				return httperrors.ToAPIError(c, op, apperr.ErrInvalidAccessToken)
			}

			xecho.UserToEchoCtx(c, user)

			return next(c)
		}
	}
}

func userFromAccessClaims(ctx context.Context, claims *entity.AccessClaims) (*entity.User, error) {
	err := validation.ValidateStructWithContext(ctx, claims,
		validation.Field(&claims.Subject, validation.Required, validation.By(xvalidation.UUIDNotNil)),
		validation.Field(&claims.Email, validation.Required),
		validation.Field(&claims.Roles, validation.Required, validation.Each(validation.Required)),
	)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, err
	}

	return &entity.User{
		ID:    userID,
		Email: claims.Email,
		Roles: claims.Roles,
	}, nil
}

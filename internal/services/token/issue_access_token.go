package token

import (
	"cmp"
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func (s *Service) IssueAccessToken(ctx context.Context, accessTokenTTL time.Duration, user *entity.User) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.IssueAccessToken")
	defer span.End()

	// A blocked user must not obtain an access token through any path — initial
	// login, refresh-token rotation, and grace-period re-issue all funnel here.
	if user.IsBlocked() {
		xlog.Warn(ctx, "refusing to issue access token for blocked user", xfield.Any("user", user.ID))
		return "", apperr.ErrUserBlocked
	}

	now := s.getNowF()

	// OIDC-провайдеры (напр. Google) не гарантируют claim `name`, а users.name —
	// NOT NULL DEFAULT ''. Без фолбэка такой юзер получил бы токен с пустым
	// user_name, который RequireAccessToken зарубит как невалидный (вечный 401).
	claims := entity.AccessClaims{
		UserName:  cmp.Or(user.Name, user.Email, "unknown"),
		UserEmail: user.Email,
		UserRoles: user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        xuuid.NewString(), // jti — для blacklist при logout
			Subject:   user.ID.String(),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = s.kid

	accessToken, err := token.SignedString(s.privateKey)
	if err != nil {
		xlog.Error(ctx, "failed to sign access token", xfield.Error(err))
		return "", err
	}

	return accessToken, nil
}

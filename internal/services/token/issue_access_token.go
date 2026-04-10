package token

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func (s *Service) IssueAccessToken(ctx context.Context, accessTokenTTL time.Duration, user *entity.User) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.IssueAccessToken")
	defer span.End()

	now := s.getNowF()
	claims := entity.AccessClaims{
		Email: user.Email,
		Roles: user.Roles,
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

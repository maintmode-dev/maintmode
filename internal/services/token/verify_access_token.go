package token

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func (s *Service) VerifyAccessToken(ctx context.Context, tokenString string) (*entity.AccessClaims, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.VerifyAccessToken")
	defer span.End()

	token, err := jwt.ParseWithClaims(tokenString, &entity.AccessClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return &s.privateKey.PublicKey, nil
	})
	if err != nil {
		xlog.Error(ctx, "failed to parse access token", xfield.Error(err))
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %w", apperr.ErrTokenExpired, err)
		}
		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessTokenToken, err)
	}

	claims, ok := token.Claims.(*entity.AccessClaims)
	if !ok || !token.Valid {
		xlog.Error(ctx, "invalid access token claims")
		return nil, apperr.ErrInvalidAccessTokenToken
	}
	return claims, nil
}

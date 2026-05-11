package jwtverifier

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

func (s *Service) VerifyAccessToken(ctx context.Context, tokenString string) (*entity.AccessClaims, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.JWTVerifier.VerifyAccessToken")
	defer span.End()

	verifyStartedAt := xtime.UTCNow()
	claims := new(entity.AccessClaims)

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		s.keyfunc.KeyfuncCtx(ctx),
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithIssuer(s.cfg.JWTIssuer),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(s.cfg.JWTLeeway),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		xlog.Error(ctx, "failed to verify access token", xfield.Error(err))

		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %w", apperr.ErrTokenExpired, err)
		}
		if errors.Is(err, jwkset.ErrKeyNotFound) && s.authUnavailable(ctx, verifyStartedAt) {
			return nil, apperr.ErrAuthUnavailable
		}
		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, err)
	}

	if !token.Valid {
		return nil, apperr.ErrInvalidAccessToken
	}

	return claims, nil
}

func (s *Service) authUnavailable(ctx context.Context, verifyStartedAt time.Time) bool {
	keys, err := s.keyfunc.Storage().KeyReadAll(ctx)
	if err != nil {
		xlog.Error(ctx, "read cached jwks keys failed", xfield.Error(err))
		return true
	}

	if len(keys) == 0 {
		return true
	}

	return s.LastRefreshFailedAt(ctx) >= verifyStartedAt.UnixNano()
}

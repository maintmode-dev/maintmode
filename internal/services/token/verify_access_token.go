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

	// The keyfunc already rejects non-ECDSA, but the parser options are what
	// make the guarantees structural rather than a property of this callback:
	// ES256 exactly (not any ECDSA curve), our own issuer, and an exp that must
	// be present — an absent one is otherwise not an error but the absence of
	// one, which verifies forever.
	//
	// WithIssuedAt is deliberately NOT set. In jwt/v5 it rejects an iat in the
	// future rather than requiring iat to exist, so without WithLeeway any clock
	// skew between the replica that issued a token and the one verifying it
	// turns logout/introspect into a 401. This package has no leeway configured;
	// jwtverifier can afford the option because it does.
	token, err := jwt.ParseWithClaims(tokenString, &entity.AccessClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return &s.privateKey.PublicKey, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		xlog.Error(ctx, "failed to parse access token", xfield.Error(err))
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %w", apperr.ErrTokenExpired, err)
		}
		return nil, fmt.Errorf("%w: %w", apperr.ErrInvalidAccessToken, err)
	}

	claims, ok := token.Claims.(*entity.AccessClaims)
	if !ok || !token.Valid {
		xlog.Error(ctx, "invalid access token claims")
		return nil, apperr.ErrInvalidAccessToken
	}
	return claims, nil
}

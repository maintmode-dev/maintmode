package token

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// JWKS returns the public key.
func (s *Service) JWKS(ctx context.Context) (entity.JWKS, error) {
	_, span := xlog.WithOperationSpan(ctx, "service.AccessToken.JWKS")
	defer span.End()

	// Uncompressed SEC 1 point: 0x04 || X || Y, each coordinate already at the
	// curve's fixed width (32 bytes for P-256), as JWK requires.
	point, err := s.privateKey.PublicKey.Bytes()
	if err != nil {
		return entity.JWKS{}, fmt.Errorf("encode public key: %w", err)
	}
	const coordBytes = 32

	return entity.JWKS{
		Keys: []entity.JWK{
			{
				Kty: "EC",
				Crv: "P-256",
				Kid: s.kid,
				Use: "sig",
				Alg: "ES256",
				X:   base64.RawURLEncoding.EncodeToString(point[1 : 1+coordBytes]),
				Y:   base64.RawURLEncoding.EncodeToString(point[1+coordBytes:]),
			},
		},
	}, nil
}

package token

import (
	"context"
	"encoding/base64"
	"math/big"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// JWKS returns the public key.
func (s *Service) JWKS(ctx context.Context) entity.JWKS {
	_, span := xlog.WithOperationSpan(ctx, "service.AccessToken.JWKS")
	defer span.End()

	pub := s.privateKey.PublicKey
	return entity.JWKS{
		Keys: []entity.JWK{
			{
				Kty: "EC",
				Crv: "P-256",
				Kid: s.kid,
				Use: "sig",
				Alg: "ES256",
				X:   base64url(pub.X),
				Y:   base64url(pub.Y),
			},
		},
	}
}

func base64url(n *big.Int) string {
	b := n.Bytes()
	// Pad to 32 bytes for P-256
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return base64.RawURLEncoding.EncodeToString(padded)
}

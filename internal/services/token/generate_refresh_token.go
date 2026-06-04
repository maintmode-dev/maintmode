package token

import (
	"context"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/utils/xcripto"
)

// refreshTokenBytes is the entropy of the raw invitation token (256 bits),
// matching the refresh-token generator.
const refreshTokenBytes = 32

// GenerateRefreshToken creates a random opaque refresh token string.
func (s *Service) GenerateRefreshToken(ctx context.Context) (raw, hashed string, err error) {
	_, span := xlog.WithOperationSpan(ctx, "service.AccessToken.GenerateRefreshToken")
	defer span.End()

	return xcripto.GenerateToken(refreshTokenBytes)
}

package token

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// GenerateRefreshToken creates a random opaque refresh token string.
func (s *Service) GenerateRefreshToken(ctx context.Context) (raw, hashed string, err error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.AccessToken.GenerateRefreshToken")
	defer span.End()

	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		xlog.Error(ctx, "failed to generate random bytes", xfield.Error(err))
		return "", "", err
	}
	raw = base64.URLEncoding.EncodeToString(b)
	return raw, xhash.HashSha256([]byte(raw)), nil
}

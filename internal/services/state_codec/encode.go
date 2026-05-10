package statecodec

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Encode sets the state's ExpiresAt and returns a signed string.
func (c *Service) Encode(ctx context.Context, s *entity.OAuthState) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.SignedStateCodec.Encode")
	defer span.End()

	s.ExpiresAt = c.nowFn().Add(c.ttl).Unix()

	payload, err := json.Marshal(s)
	if err != nil {
		xlog.Error(ctx, "failed to marshal state", xfield.Error(err))
		return "", fmt.Errorf("marshal state: %w", err)
	}

	encPayload := base64.RawURLEncoding.EncodeToString(payload)

	sig := c.sign([]byte(encPayload))
	encSig := base64.RawURLEncoding.EncodeToString(sig)

	return fmt.Sprintf("%s.%s", encPayload, encSig), nil
}

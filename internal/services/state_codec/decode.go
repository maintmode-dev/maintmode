package statecodec

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Decode verifies signature and expiry, then returns the payload.
// Returns apperr.ErrOAuthStateTampered for any signature/format failure,
// and apperr.ErrOAuthStateExpired when the state's expiry has passed.
func (c *Service) Decode(ctx context.Context, raw string) (*entity.OAuthState, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "service.SignedStateCodec.Decode")
	defer span.End()

	encPayload, encSig, err := extractParts(raw)
	if err != nil {
		xlog.Error(ctx, "failed to extract parts", xfield.Error(err))
		return nil, fmt.Errorf("%w: %s", apperr.ErrOAuthStateTampered, err.Error())
	}

	ok, err := c.compareSign(encPayload, encSig)
	if err != nil || !ok {
		xlog.Error(ctx, "failed to verify signature", xfield.Error(err), xfield.Bool("compare", ok))
		return nil, apperr.ErrOAuthStateTampered
	}

	payload, err := base64.RawURLEncoding.DecodeString(encPayload)
	if err != nil {
		xlog.Error(ctx, "failed to decode payload", xfield.Error(err))
		return nil, apperr.ErrOAuthStateTampered
	}

	state := new(entity.OAuthState)
	if err := json.Unmarshal(payload, state); err != nil {
		xlog.Error(ctx, "failed to unmarshal payload", xfield.Error(err))
		return nil, apperr.ErrOAuthStateTampered
	}

	if state.ExpiresAt <= 0 || c.nowFn().Unix() > state.ExpiresAt {
		xlog.Error(ctx, "state expired")
		return nil, apperr.ErrOAuthStateExpired
	}

	return state, nil
}

func (c *Service) compareSign(encPayload, encSig string) (bool, error) {
	actualSig, err := base64.RawURLEncoding.DecodeString(encSig)
	if err != nil {
		return false, apperr.ErrOAuthStateTampered
	}

	expectedSig := c.sign([]byte(encPayload))
	if subtle.ConstantTimeCompare(expectedSig, actualSig) != 1 {
		return false, apperr.ErrOAuthStateTampered
	}

	return true, nil
}

func extractParts(raw string) (encPayload, encSig string, err error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid state format")
	}

	return parts[0], parts[1], nil
}

package statecodec

import (
	"crypto/hmac"
	"crypto/sha256"
	"time"

	"github.com/ruko1202/maintmode/internal/utils/xtime"
)

// Service encodes and verifies OAuth state with an HMAC-SHA256 signature
// and an embedded expiry. Format: b64url(payloadJSON) + "." + b64url(hmac).
type Service struct {
	secret []byte
	ttl    time.Duration
	nowFn  func() time.Time
}

// NewService returns a codec that signs and verifies OAuthState.
// secret must be at least 16 bytes; nowFn is used for expiry checks (defaults to time.Now).
func NewService(secret []byte, ttl time.Duration) *Service {
	return &Service{
		secret: secret,
		ttl:    ttl,
		nowFn:  xtime.UTCNow,
	}
}

func (c *Service) sign(data []byte) []byte {
	h := hmac.New(sha256.New, c.secret)
	h.Write(data)
	return h.Sum(nil)
}

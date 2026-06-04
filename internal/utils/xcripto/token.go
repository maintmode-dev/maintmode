package xcripto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// GenerateToken returns a random opaque token (delivered in the email link) and
// its sha256 hash (stored in the DB). The raw token is never persisted.
func GenerateToken(length int) (raw, hash string, err error) {
	b := make([]byte, length)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token bytes: %w", err)
	}
	raw = base64.URLEncoding.EncodeToString(b)
	return raw, xhash.HashSha256([]byte(raw)), nil
}

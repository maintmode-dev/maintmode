package xhash

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashSha256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

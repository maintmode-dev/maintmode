package xcripto

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ruko1202/maintmode/internal/utils/xhash"
)

// otpCodeDigits is the length of a one-time code. Six digits is the shape users
// expect from an emailed code and the one the design doc writes; the control on
// guessing is the per-code attempt ceiling, not the size of the space.
const otpCodeDigits = 6

// otpCodeSpace is the exclusive upper bound of a code: 10^6.
const otpCodeSpace = 1_000_000

// otpRejectionCeiling is the largest multiple of otpCodeSpace that fits in a
// uint32. Samples at or above it are discarded rather than folded, which is what
// keeps every code equally likely -- see GenerateOTPCode.
const otpRejectionCeiling = (1 << 32) - ((1 << 32) % otpCodeSpace)

// GenerateOTPCode returns a random six-digit one-time code and its sha256 hex
// digest. The raw code goes in the email; only the digest is ever stored.
//
// The code is a zero-padded string, not a number. Roughly one code in ten is
// below 100000, and formatting such a value without padding would email five
// digits (or fewer) while the user is asked for six -- and the digest would be
// of a different string than the one they type back.
//
// The digest is taken of that same six-character string, matching GenerateToken,
// which hashes the string it returns rather than the bytes behind it. Verification
// hashes what the user typed, so both sides must agree on what was hashed; this is
// the half of that contract this package owns.
//
// Sampling rejects rather than reduces. 2^32 is not a multiple of 10^6, so a
// plain modulo would make the lowest 967296 codes fractionally more likely than
// the rest -- a small bias, but a free one to avoid, and bias in an
// authentication secret is not the place to spend a shortcut.
func GenerateOTPCode() (code, hash string, err error) {
	return generateOTPCode(rand.Reader)
}

// generateOTPCode is GenerateOTPCode with the entropy source injected, so a test
// can drive the rejection branch with bytes that would otherwise never appear.
func generateOTPCode(entropy io.Reader) (code, hash string, err error) {
	var buf [4]byte

	for {
		if _, err = io.ReadFull(entropy, buf[:]); err != nil {
			return "", "", fmt.Errorf("generate otp code bytes: %w", err)
		}

		n := binary.BigEndian.Uint32(buf[:])
		if n >= otpRejectionCeiling {
			continue
		}

		code = fmt.Sprintf("%0*d", otpCodeDigits, n%otpCodeSpace)
		return code, xhash.HashSha256([]byte(code)), nil
	}
}

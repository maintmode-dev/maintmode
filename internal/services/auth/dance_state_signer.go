package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const danceSecretBytes = 32

// newDanceSecret mints one dance secret: the state, the PKCE verifier or the
// one-time code.
//
// RawURLEncoding, not the padded form xcripto's helper uses: a trailing "=" is
// invalid in an unquoted cookie value, and neither http.SetCookie nor c.Cookie
// encodes it. RFC 7636 requires the same unpadded alphabet for a PKCE verifier,
// so one helper serves all three secrets.
func newDanceSecret() (string, error) {
	b := make([]byte, danceSecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate dance secret: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// danceStateSigner signs and verifies the state the browser carries, so the
// dance needs no server-side store: /start hands out a signature, /callback
// re-derives it from what the provider returned.
//
// It buys authenticity and a deadline, NOT uniqueness. The signature is
// deterministic, so a captured (state, cookie) pair verifies as often as it is
// presented inside the window; single-use comes from the provider burning the
// authorization code instead.
type danceStateSigner struct {
	key []byte
}

// stateSignatureLabel lets a future scheme coexist with signatures already in
// flight rather than colliding with them.
const stateSignatureLabel = "oauth-state-v1"

// stateSignatureSeparator is safe because neither signed field can contain it:
// the state is base64url and the provider comes from a validated allow-list.
const stateSignatureSeparator = "."

// newDanceStateSigner derives the signing key from the two secrets, so rotating
// EITHER one ends every dance in flight with no store to purge.
//
// jwtPrivateKey must be config.JWT.PrivateKey as the hex STRING, never the
// parsed key or its D bytes: at exactly 64 characters it sits on HMAC-SHA256's
// block boundary and is zero-padded, while the decoded bytes are padded
// differently. Swapping the form silently invalidates dances in flight, and a
// sign-then-verify test inside one process passes under either — hence the test
// that pins it.
//
// The secrets go through a KDF rather than concatenation: with both operands
// variable-length, ("ab","cdef") and ("abc","def") derive the same key, so a
// client-secret rotation that shifts the boundary could reproduce a
// pre-rotation key.
//
// Operational coupling: rotating the JWT key is already the "sign everyone out"
// lever and now also ends dances in flight, so do not sequence one into the
// middle of an OAuth rollout.
func newDanceStateSigner(jwtPrivateKey, clientSecret string) danceStateSigner {
	mac := hmac.New(sha256.New, []byte(jwtPrivateKey))
	mac.Write([]byte(stateSignatureLabel + clientSecret))

	return danceStateSigner{key: mac.Sum(nil)}
}

// Sign returns the cookie value "exp.signature", the signature covering the
// provider, the state and that same exp.
//
// The expiry is inside the signed material because cookie MaxAge is an
// instruction the browser may ignore and an attacker ignores by definition.
func (s danceStateSigner) Sign(provider, state string, exp time.Time) string {
	unix := strconv.FormatInt(exp.Unix(), 10)

	return unix + stateSignatureSeparator + s.signature(provider, state, unix)
}

// Verify reports whether cookieValue authenticates this state for this provider
// and has not expired. Every failure is the same false; the causes stay tellable
// apart only in the audit trail.
func (s danceStateSigner) Verify(cookieValue, provider, state string, now time.Time) bool {
	// Cut splits on the FIRST separator, so anything appended after an otherwise
	// valid cookie lands in the signature half and fails the comparison below
	// rather than being quietly ignored.
	unix, signature, found := strings.Cut(cookieValue, stateSignatureSeparator)
	if !found {
		return false
	}

	exp, err := strconv.ParseInt(unix, 10, 64)
	if err != nil {
		return false
	}

	// No skew window: one instance both signs and verifies, and any allowance
	// here would silently widen the lifetime.
	if !now.Before(time.Unix(exp, 0)) {
		return false
	}

	// Must stay hmac.Equal: == returns the same booleans, so no test can catch
	// the swap — only the timing differs. This comment is the guard, as it is in
	// bootstrapauth.Authenticate and otp.verify.
	return hmac.Equal([]byte(signature), []byte(s.signature(provider, state, unix)))
}

func (s danceStateSigner) signature(provider, state, unix string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(provider + stateSignatureSeparator + state + stateSignatureSeparator + unix))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

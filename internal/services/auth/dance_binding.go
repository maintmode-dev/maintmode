package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// The browser binding of an OAuth dance.
//
// The BFF mints a nonce, keeps it in a cookie of its own, and opens /start with
// binding = base64url(SHA-256(nonce)). The binding rides the dance in a cookie
// and lands on the one-time code; redeeming the code takes the nonce itself.
// Only the BFF serving the browser that started the dance holds that nonce, so
// a code -- or a link code -- carried to another browser redeems nothing.
//
// The hash, not the nonce, travels through the dance: the /start URL passes
// through the browser history and the gateway's logs, and what it reveals must
// not be enough to redeem anything.

// validDanceBinding reports whether binding has the shape of a SHA-256 digest
// in unpadded base64url. Checked at /start, so a dance that could never be
// redeemed is refused before the person reaches the provider.
func validDanceBinding(binding string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(binding)

	return err == nil && len(raw) == sha256.Size
}

// danceBindingProven reports whether proof is the nonce binding was derived
// from. An empty side never proves anything: a code minted without a binding is
// not a code anyone may redeem without one.
func danceBindingProven(binding, proof string) bool {
	if binding == "" || proof == "" {
		return false
	}

	sum := sha256.Sum256([]byte(proof))
	want := base64.RawURLEncoding.EncodeToString(sum[:])

	return subtle.ConstantTimeCompare([]byte(want), []byte(binding)) == 1
}

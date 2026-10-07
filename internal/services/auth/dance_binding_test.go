package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

// testDanceNonce is the nonce a test BFF keeps; testDanceBinding is what it
// hands /start.
const testDanceNonce = "test-bff-binding-nonce"

var testDanceBinding = func() string {
	sum := sha256.Sum256([]byte(testDanceNonce))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}()

func TestDanceBindingProven(t *testing.T) {
	t.Parallel()

	assert.True(t, danceBindingProven(testDanceBinding, testDanceNonce))

	assert.False(t, danceBindingProven(testDanceBinding, "another-browser"))
	assert.False(t, danceBindingProven(testDanceBinding, ""),
		"no proof never proves")
	assert.False(t, danceBindingProven("", ""),
		"a code minted without a binding is not redeemable without one")
	assert.False(t, danceBindingProven("", testDanceNonce))
	assert.False(t, danceBindingProven(testDanceNonce, testDanceNonce),
		"the binding is the HASH of the nonce, so the nonce does not prove itself")
}

func TestValidDanceBinding(t *testing.T) {
	t.Parallel()

	assert.True(t, validDanceBinding(testDanceBinding))

	for _, bad := range []string{"", "short", testDanceBinding + "=", testDanceBinding + "AA", "not a digest!"} {
		assert.False(t, validDanceBinding(bad), bad)
	}
}

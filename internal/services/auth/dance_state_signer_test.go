package auth

import (
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The key the signature is seeded from, and a state value in the base64url
// alphabet the real one uses.
const (
	testJWTKey       = "1be2f1f68285c972b750b7718b00d5453f2c08f88c7894d1b9013f75a439de20"
	testStateValue   = "Zm9vYmFyLXN0YXRlLXZhbHVl"
	testStateExpFrom = 10 * time.Minute
)

func testSigner() danceStateSigner {
	return newDanceStateSigner(testJWTKey)
}

// TestStateSignatureRoundTrips is the happy path: what Sign produced verifies
// for the same provider and state inside the window.
func TestStateSignatureRoundTrips(t *testing.T) {
	t.Parallel()

	now := time.Now()
	signer := testSigner()

	cookie := signer.Sign("google", testStateValue, now.Add(testStateExpFrom))

	assert.True(t, signer.Verify(cookie, "google", testStateValue, now))
}

// TestStateSignatureIsNotTheState guards the shape of the cookie itself.
//
// A rewrite that "simplifies" the cookie down to the state it signs would still
// pass a round-trip test — Sign and Verify would agree — while handing whoever
// reads the redirect URL everything they need to forge a callback. The cookie
// must not contain the state, in any form.
func TestStateSignatureIsNotTheState(t *testing.T) {
	t.Parallel()

	cookie := testSigner().Sign("google", testStateValue, time.Now().Add(testStateExpFrom))

	assert.NotContains(t, cookie, testStateValue,
		"the cookie must carry a signature over the state, never the state itself")
}

// TestStateSignatureRefusesTamperedInput covers every way the three signed
// fields can fail to match.
func TestStateSignatureRefusesTamperedInput(t *testing.T) {
	t.Parallel()

	now := time.Now()
	signer := testSigner()
	cookie := signer.Sign("google", testStateValue, now.Add(testStateExpFrom))

	tests := map[string]struct {
		provider string
		state    string
	}{
		// RUK-293 and RUK-295 add providers. A dance begun for one must not be
		// completable as another, whose id_token this instance would verify
		// against different keys.
		"another provider": {provider: "github", state: testStateValue},
		"another state":    {provider: "google", state: "some-other-state"},
		"empty state":      {provider: "google", state: ""},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.False(t, signer.Verify(cookie, tt.provider, tt.state, now))
		})
	}
}

// TestStateSignatureExpires proves the deadline is enforced by the SERVER.
//
// Cookie MaxAge is an instruction to the browser, not a rule anyone enforces: an
// attacker who keeps the cookie value replays it whenever they like. The expiry
// is inside the signed material precisely so this check exists, and a mutation
// dropping exp from the signature has to fail here.
func TestStateSignatureExpires(t *testing.T) {
	t.Parallel()

	now := time.Now()
	signer := testSigner()
	cookie := signer.Sign("google", testStateValue, now.Add(testStateExpFrom))

	assert.True(t, signer.Verify(cookie, "google", testStateValue, now.Add(testStateExpFrom-time.Second)),
		"a signature one second before its expiry is still good")
	assert.False(t, signer.Verify(cookie, "google", testStateValue, now.Add(testStateExpFrom+time.Second)),
		"a signature past its expiry must be refused, whatever the browser did with MaxAge")
}

// TestStateSignatureCoversItsOwnExpiry is the test that gives `exp` its reason
// to be inside the signed material, and it is a different claim from the one
// above.
//
// Verify reads exp FROM the cookie and checks the deadline before comparing
// signatures, so the expiry test passes whether or not exp is signed. What only
// this test catches: the cookie is entirely attacker-controlled, so if exp is
// not covered by the signature, anyone holding a captured cookie can rewrite the
// timestamp to any future value and the signature still matches. The 10-minute
// window would then be decorative.
//
// Dropping exp from the signed message must fail here.
func TestStateSignatureCoversItsOwnExpiry(t *testing.T) {
	t.Parallel()

	now := time.Now()
	signer := testSigner()
	cookie := signer.Sign("google", testStateValue, now.Add(testStateExpFrom))

	_, signature, found := strings.Cut(cookie, ".")
	require.True(t, found)

	// The same signature, re-presented with a far-future expiry — exactly what an
	// attacker who kept the cookie would send.
	extended := strconv.FormatInt(now.Add(365*24*time.Hour).Unix(), 10) + "." + signature

	assert.False(t, signer.Verify(extended, "google", testStateValue, now),
		"an attacker-rewritten expiry must break the signature, or the lifetime is unenforced")
}

// TestStateSignatureDiesWithTheIssuerKey pins the one remaining rotation lever.
//
// It used to be two: the client secret was mixed into the key so that rotating
// either secret ended every dance in flight. With providers configured per
// instance there is no single client secret to mix, and picking one instance's
// would make the other instances' dances depend on a credential unrelated to
// them. Rotating a provider's client secret therefore no longer invalidates
// state in flight -- only rotating the issuer key does.
func TestStateSignatureDiesWithTheIssuerKey(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exp := now.Add(testStateExpFrom)
	cookie := testSigner().Sign("google", testStateValue, exp)

	// Rotating this key is already the "sign everyone out" lever; it also ends
	// dances in flight.
	rotated := newDanceStateSigner(
		"0000000000000000000000000000000000000000000000000000000000000001")
	assert.False(t, rotated.Verify(cookie, "google", testStateValue, now))
}

// TestStateSignatureIsUnambiguousAcrossProviders pins that the MAC encoding is
// injective — one signature cannot be reinterpreted as a different field set.
//
// Joined with a bare separator these two are the SAME byte string:
//
//	"a" + "." + "b.<state>"   ==   "a.b" + "." + "<state>"
//
// so a dance for provider "a" would produce a cookie that verifies for provider
// "a.b". Length prefixing is what breaks the tie. config.ValidateInstanceKey
// happens to forbid a dot in an instance name, which would also make this
// unreachable — but a charset rule is not where a signature's unforgeability
// should live, and this test fails if the prefixing is removed in favor of
// relying on it.
func TestStateSignatureIsUnambiguousAcrossProviders(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exp := now.Add(testStateExpFrom)

	cookie := testSigner().Sign("a", "b."+testStateValue, exp)
	assert.False(t, testSigner().Verify(cookie, "a.b", testStateValue, now),
		"a signature for provider %q must not verify for provider %q", "a", "a.b")
}

// TestStateSignatureRefusesMalformedCookies covers what arrives from the wire.
//
// The cookie value is entirely attacker-controlled, so every shape that is not
// "exp.signature" has to be refused rather than parsed optimistically. A
// forgotten error branch here reads as an accept.
func TestStateSignatureRefusesMalformedCookies(t *testing.T) {
	t.Parallel()

	now := time.Now()
	signer := testSigner()
	valid := signer.Sign("google", testStateValue, now.Add(testStateExpFrom))
	signature := valid[strings.Index(valid, ".")+1:]

	tests := map[string]string{
		"empty":              "",
		"no separator":       signature,
		"only a separator":   ".",
		"missing signature":  strconv.FormatInt(now.Add(testStateExpFrom).Unix(), 10) + ".",
		"missing exp":        "." + signature,
		"non-numeric exp":    "not-a-number." + signature,
		"extra separator":    valid + ".extra",
		"forged signature":   strconv.FormatInt(now.Add(testStateExpFrom).Unix(), 10) + ".AAAA",
		"whitespace padding": " " + valid,
	}

	for name, cookie := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.False(t, signer.Verify(cookie, "google", testStateValue, now))
		})
	}
}

// TestStateSignatureSeedIsTheConfiguredString pins the FORM of the JWT seed.
//
// config.JWT.PrivateKey is 64 hex characters — exactly HMAC-SHA256's block size,
// so it is zero-padded rather than hashed, while the parsed key's 32 raw bytes
// are a different input entirely. Both are secure; they are not the same key. A
// later cleanup from one form to the other would silently invalidate every dance
// in flight, and a sign/verify test inside one process passes under either.
//
// This test is what makes that cleanup fail: the hex string and the bytes it
// decodes to must not produce the same signature.
func TestStateSignatureSeedIsTheConfiguredString(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exp := now.Add(testStateExpFrom)

	fromString := newDanceStateSigner(testJWTKey).
		Sign("google", testStateValue, exp)

	decoded, err := hex.DecodeString(testJWTKey)
	require.NoError(t, err)

	fromBytes := newDanceStateSigner(string(decoded)).
		Sign("google", testStateValue, exp)

	assert.NotEqual(t, fromString, fromBytes,
		"the hex string and its decoded bytes are different keys; the config string is the pinned one")
}

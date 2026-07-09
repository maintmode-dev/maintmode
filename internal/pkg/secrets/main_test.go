package secrets

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Format-v2 golden fixture for TestAESCipher_DecryptGoldenVector: a serialized
// cleartext DEK-keyset and an envelope of "xoxb-golden-secret" sealed under it
// with SecretAAD("slack", "bot_token"). See the test for the regeneration
// policy (in short: don't).
const (
	goldenDEKHex      = "08c5fecf880c12640a580a30747970652e676f6f676c65617069732e636f6d2f676f6f676c652e63727970746f2e74696e6b2e41657347636d4b657912221a200d32341558aa547f00d179f1c6b60e15c9c07d92b5d523fb2c59e68ba6fdb19a1801100118c5fecf880c2001"
	goldenEnvelopeHex = "01c113ff4575d7fb082db2f5bd19f620f88b2ec22ec8d01c6d90b815fdfc0d8d0373c9448081cecf90894d9cf0c8428ecf5b55"
)

// Wrapped-DEK counterpart: the golden DEK above wrapped under a fixed KEK. It
// pins the SECOND persisted format — data_keys.encrypted_dek (Tink encrypted
// keyset write + DEKWrapAAD over the KEK URI) — which the secret-envelope
// vector does not cover. Same regeneration policy: don't.
const (
	goldenKEKHex        = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	goldenKEKURI        = "local-kms://golden-1"
	goldenWrappedDEKHex = "12880167bdf3bac1ebd78b564c9d816417d4a0629e6565dd87a780b34a09f6d4fe68bf2b7fe2c6bfacc70b90c643b477366c4d6182c5a0c87c0643d2cd38d97bcb99b8e6ddffa4d670068da4c17a427355c11e307beef05c555a43a1e393da16ed894ed756795da3ae611d6278dc800be8b6825045db74e93bfd5c209dd26c05cddada35bf31486a48ab13"
)

// tinkPrefixLen is the length of Tink's output prefix on an AEAD envelope:
// 1 version byte + 4 key-id bytes. The 12-byte GCM IV follows it. Test-only
// knowledge: production code never parses the envelope, Tink does.
const tinkPrefixLen = 5

var (
	// testAAD is a representative binding for the cipher tests below. The specific
	// value doesn't matter here — only that Encrypt and Decrypt agree on it.
	testAAD = SecretAAD("slack", "bot_token")
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

// testDEK returns a fresh serialized DEK-keyset for cipher tests.
func testDEK(t *testing.T) []byte {
	t.Helper()
	dek, err := GenerateDEK()
	require.NoError(t, err)
	return dek
}

// kekURI returns a distinct local KEK URI for tests.
func kekURI(n int) string { return fmt.Sprintf("local-kms://kek-%d", n) }

// hexKey returns a distinct, valid 32-byte KEK as a hex string. b seeds the
// first byte so callers get different keys without pulling in randomness.
func hexKey(b byte) string {
	key := make([]byte, keyLen)
	for i := range key {
		key[i] = b + byte(i)
	}
	return hex.EncodeToString(key)
}

// requireSameDEK asserts two serialized DEK-keysets are functionally the same
// key material — the same check rotation relies on before overwriting a wrap.
func requireSameDEK(t *testing.T, want, got []byte) {
	t.Helper()
	require.NoError(t, EquivalentDEKs(want, got))
}

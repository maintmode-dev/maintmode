package secrets

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	tinksubtle "github.com/tink-crypto/tink-go/v2/aead/subtle"
	"github.com/tink-crypto/tink-go/v2/core/registry"
	"github.com/tink-crypto/tink-go/v2/tink"

	"github.com/ruko1202/maintmode/internal/utils/xcollection"
)

const (
	// LocalKEKScheme prefixes every locally-provided KEK URI (local-kms://<id>).
	// The scheme is what routes a URI to the local client; a cloud KMS deployment
	// swaps it for the provider's scheme (gcp-kms://…, aws-kms://…) and registers
	// that provider's client instead — nothing else changes.
	LocalKEKScheme = "local-kms://"
	// keyLen is the required local KEK size for AES-256 (32 bytes).
	keyLen = 32
)

// localKMSClient serves KEK AEADs for local-kms:// URIs from operator-supplied
// raw keys (config crypto.local_keys). It implements registry.KMSClient — the
// same interface Tink's cloud KMS clients implement — so the Keyring cannot
// tell a local KEK from a KMS-held one. That interface seam is the entire KMS
// migration path.
//
// Key bytes live only in memory. The client never logs or formats them; do not
// add a Stringer that exposes keys.
type localKMSClient struct {
	aeads *xcollection.MUMap[string, tink.AEAD] // kek URI -> AES-256-GCM primitive
}

// NewLocalKMSClient validates and loads hex-encoded local KEKs keyed by their
// local-kms:// URI, failing fast on anything unusable: an empty set, a URI
// without the local-kms:// scheme, or a key that is not valid hex, not 32
// bytes, or all-zero (placeholder/unset). A weak or missing KEK must abort
// startup rather than silently protect secrets with an unusable key.
func NewLocalKMSClient(keys map[string]string) (registry.KMSClient, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("secrets: no local keks configured")
	}

	emptyKeySample := make([]byte, keyLen)

	aeads := xcollection.NewMUMap[string, tink.AEAD]()
	for uri, hexKey := range keys {
		if !strings.HasPrefix(uri, LocalKEKScheme) {
			return nil, fmt.Errorf("secrets: local kek %q must use the %s scheme", uri, LocalKEKScheme)
		}
		key, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("secrets: local kek %q is not valid hex: %w", uri, err)
		}
		if len(key) != keyLen {
			return nil, fmt.Errorf("secrets: local kek %q must decode to %d bytes, got %d", uri, keyLen, len(key))
		}

		if bytes.Equal(key, emptyKeySample) {
			return nil, fmt.Errorf("secrets: local kek %q is all-zero (placeholder or unset)", uri)
		}

		primitive, err := tinksubtle.NewAESGCM(key)
		if err != nil {
			return nil, fmt.Errorf("secrets: local kek %q: %w", uri, err)
		}
		aeads.Set(uri, primitive)
	}
	return &localKMSClient{
		aeads: aeads,
	}, nil
}

func (c *localKMSClient) Supported(keyURI string) bool {
	return c.aeads.Has(keyURI)
}

func (c *localKMSClient) GetAEAD(keyURI string) (tink.AEAD, error) {
	primitive, ok := c.aeads.Get(keyURI)
	if !ok {
		return nil, fmt.Errorf("secrets: unknown kek %q", keyURI)
	}
	return primitive, nil
}

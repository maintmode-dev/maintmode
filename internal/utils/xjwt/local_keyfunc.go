package xjwt

import (
	"context"
	"crypto/ecdsa"
	"fmt"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
)

// NewLocalKeyFunc builds a keyfunc over the issuer's own public key. The merged
// process signs and verifies with the same key, so fetching it over HTTP from
// ourselves would only add a startup race and a network dependency.
//
// It takes an already-parsed public key: the private half never enters this
// package, and a malformed key is rejected at config load, not here.
func NewLocalKeyFunc(ctx context.Context, pub *ecdsa.PublicKey, kid string) (keyfunc.Keyfunc, error) {
	jwk, err := jwkset.NewJWKFromKey(pub, jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{
			KID: kid,
			USE: jwkset.UseSig,
			ALG: jwkset.AlgES256,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("build local jwk: %w", err)
	}

	storage := jwkset.NewMemoryStorage()
	if err := storage.KeyWrite(ctx, jwk); err != nil {
		return nil, fmt.Errorf("write local jwk: %w", err)
	}

	return keyfunc.New(keyfunc.Options{Ctx: ctx, Storage: storage})
}

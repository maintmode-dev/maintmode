//go:build api

package api

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/test/api/client/client/auth"
	"github.com/ruko1202/maintmode/test/api/client/models"
)

func TestAuthAPI_JWKS(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	params := auth.NewGetAPIV1WellKnownJwksJSONParams().WithContext(ctx)

	resp, err := apiClient.Auth.GetAPIV1WellKnownJwksJSON(params)
	require.NoError(t, err)
	require.NotNil(t, resp.Payload)
	require.NotEmpty(t, resp.Payload.Keys, "JWKS must contain at least one key")

	for _, jwk := range resp.Payload.Keys {
		pubKey := buildECPublicKey(t, jwk)
		require.NotNil(t, pubKey, "should build a valid EC public key from JWK kid=%s", jwk.Kid)
	}
}

func buildECPublicKey(t *testing.T, jwk *models.EntityJWK) *ecdsa.PublicKey {
	t.Helper()

	require.Equal(t, "EC", jwk.Kty, "expected EC key type, got %s", jwk.Kty)
	require.NotEmpty(t, jwk.X, "JWK X coordinate must not be empty")
	require.NotEmpty(t, jwk.Y, "JWK Y coordinate must not be empty")
	require.NotEmpty(t, jwk.Crv, "JWK curve must not be empty")

	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	require.NoError(t, err, "failed to decode X coordinate")

	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	require.NoError(t, err, "failed to decode Y coordinate")

	var (
		ecdhCurve ecdh.Curve
		curve     elliptic.Curve
	)

	switch jwk.Crv {
	case "P-256":
		ecdhCurve = ecdh.P256()
		curve = elliptic.P256()
	case "P-384":
		ecdhCurve = ecdh.P384()
		curve = elliptic.P384()
	case "P-521":
		ecdhCurve = ecdh.P521()
		curve = elliptic.P521()
	default:
		t.Fatalf("unsupported curve: %s", jwk.Crv)
	}

	encodedPoint := make([]byte, 1, 1+len(xBytes)+len(yBytes))
	encodedPoint[0] = 4
	encodedPoint = append(encodedPoint, xBytes...)
	encodedPoint = append(encodedPoint, yBytes...)
	_, err = ecdhCurve.NewPublicKey(encodedPoint)
	require.NoError(t, err, "point (X, Y) must be on curve %s", jwk.Crv)

	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)

	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
}

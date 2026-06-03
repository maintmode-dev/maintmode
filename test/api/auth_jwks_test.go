//go:build api

package api

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"math/big"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	authclient "github.com/ruko1202/maintmode/internal/pkg/generated/clients/auth"
)

func TestAuthAPI_JWKS(t *testing.T) {
	ctx := context.Background()

	apiClient := setupAuthTestClient()

	resp, err := apiClient.GetApiV1WellKnownJwksJsonWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), "unexpected status: %s", resp.Body)
	require.NotNil(t, resp.JSON200)

	keys := lo.FromPtr(resp.JSON200.Keys)
	require.NotEmpty(t, keys, "JWKS must contain at least one key")

	for _, jwk := range keys {
		pubKey := buildECPublicKey(t, jwk)
		require.NotNil(t, pubKey, "should build a valid EC public key from JWK kid=%s", lo.FromPtr(jwk.Kid))
	}
}

func buildECPublicKey(t *testing.T, jwk authclient.EntityJWK) *ecdsa.PublicKey {
	t.Helper()

	require.Equal(t, "EC", lo.FromPtr(jwk.Kty), "expected EC key type, got %s", lo.FromPtr(jwk.Kty))
	require.NotEmpty(t, lo.FromPtr(jwk.X), "JWK X coordinate must not be empty")
	require.NotEmpty(t, lo.FromPtr(jwk.Y), "JWK Y coordinate must not be empty")
	require.NotEmpty(t, lo.FromPtr(jwk.Crv), "JWK curve must not be empty")

	xBytes, err := base64.RawURLEncoding.DecodeString(lo.FromPtr(jwk.X))
	require.NoError(t, err, "failed to decode X coordinate")

	yBytes, err := base64.RawURLEncoding.DecodeString(lo.FromPtr(jwk.Y))
	require.NoError(t, err, "failed to decode Y coordinate")

	var (
		ecdhCurve ecdh.Curve
		curve     elliptic.Curve
	)

	switch lo.FromPtr(jwk.Crv) {
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
		t.Fatalf("unsupported curve: %s", lo.FromPtr(jwk.Crv))
	}

	encodedPoint := make([]byte, 1, 1+len(xBytes)+len(yBytes))
	encodedPoint[0] = 4
	encodedPoint = append(encodedPoint, xBytes...)
	encodedPoint = append(encodedPoint, yBytes...)
	_, err = ecdhCurve.NewPublicKey(encodedPoint)
	require.NoError(t, err, "point (X, Y) must be on curve %s", lo.FromPtr(jwk.Crv))

	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)

	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
}

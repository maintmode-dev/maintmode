package jwtverifier

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestMain(m *testing.M) {
	code := m.Run()

	os.Exit(code)
}

type jwkState struct {
	key      *ecdsa.PrivateKey
	kid      string
	requests atomic.Int64
}

func newJWKSServer(t *testing.T, state *jwkState) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		state.requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{jwkFromKey(state.key, state.kid)},
		}))
	}))
	t.Cleanup(server.Close)

	return server
}

//nolint:staticcheck
func jwkFromKey(key *ecdsa.PrivateKey, kid string) map[string]any {
	return map[string]any{
		"kty": "EC",
		"crv": "P-256",
		"kid": kid,
		"use": "sig",
		"alg": "ES256",
		"x":   base64url(key.PublicKey.X),
		"y":   base64url(key.PublicKey.Y),
	}
}

func base64url(n *big.Int) string {
	b := n.Bytes()
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return base64.RawURLEncoding.EncodeToString(padded)
}

func newTestVerifier(ctx context.Context, t *testing.T, jwksURL string) *Service {
	t.Helper()

	verifier, err := NewService(ctx, config.JWTVerifierConfig{
		JWTIssuer:                 testIssuer,
		JWKSURL:                   jwksURL,
		JWKSRefreshInterval:       time.Hour,
		JWKSHTTPTimeout:           10 * time.Second,
		JWTLeeway:                 30 * time.Second,
		JWKSUnknownKIDRefreshRate: 5 * time.Minute,
		JWKSUnknownKIDWaitMax:     10 * time.Second,
	})
	require.NoError(t, err)

	return verifier
}

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return key
}

func signTestToken(t *testing.T, key *ecdsa.PrivateKey, kid, issuer string, expiresAt time.Time, roles []entity.Role) string {
	t.Helper()

	claims := entity.AccessClaims{
		UserEmail: "alice@example.com",
		UserRoles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   uuid.NewString(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(key)
	require.NoError(t, err)

	return signed
}

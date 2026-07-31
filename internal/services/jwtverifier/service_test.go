package jwtverifier

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// A key that cannot be parsed must surface as an error from the constructor.
// The alternative is worse than it looks: NewLocalKeyFunc dereferences the key
// inside the jwkset library, so a swallowed error here turns a config typo into
// a nil-pointer panic during service wiring rather than a startup message.
func TestNewService_UnusableSigningKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name    string
		key     string
		wantErr string
	}{
		{name: "not hex", key: "zzzz", wantErr: "failed to decode private key"},
		{name: "empty", key: "", wantErr: "failed to parse private key"},
		{name: "wrong length", key: "00", wantErr: "failed to parse private key"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.NotPanics(t, func() {
				_, err := NewService(
					ctx,
					config.JWTVerifierConfig{JWTIssuer: testIssuer},
					config.JWT{PrivateKey: tc.key, Kid: "kid-1"},
				)
				require.ErrorContains(t, err, "failed to derive jwt public key")
				require.ErrorContains(t, err, tc.wantErr)
			})
		})
	}
}

// newTestVerifier builds the verifier the way production does: the key comes from
// config.JWT, not from the JWKS URL. jwksURL is still passed so tests can point it
// at a server that fails on contact and prove the network path is gone.
func newTestVerifier(ctx context.Context, t *testing.T, jwksURL string, key *ecdsa.PrivateKey, kid string) *Service {
	t.Helper()

	verifier, err := NewService(ctx, config.JWTVerifierConfig{
		JWTIssuer:                 testIssuer,
		JWKSURL:                   jwksURL,
		JWKSRefreshInterval:       time.Hour,
		JWKSHTTPTimeout:           10 * time.Second,
		JWTLeeway:                 30 * time.Second,
		JWKSUnknownKIDRefreshRate: 5 * time.Minute,
		JWKSUnknownKIDWaitMax:     10 * time.Second,
	}, testJWTConfig(key, kid))
	require.NoError(t, err)

	return verifier
}

func testJWTConfig(key *ecdsa.PrivateKey, kid string) config.JWT {
	return config.JWT{
		PrivateKey: hex.EncodeToString(key.D.FillBytes(make([]byte, 32))),
		Issuer:     testIssuer,
		Kid:        kid,
	}
}

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return key
}

func signTestToken(t *testing.T, key *ecdsa.PrivateKey, kid, issuer string, expiresAt time.Time, roles []entity.Role) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodES256, testClaims(issuer, expiresAt, roles))
	token.Header["kid"] = kid

	signed, err := token.SignedString(key)
	require.NoError(t, err)

	return signed
}

func testClaims(issuer string, expiresAt time.Time, roles []entity.Role) entity.AccessClaims {
	return entity.AccessClaims{
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
}

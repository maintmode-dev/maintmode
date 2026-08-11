package googleoauth_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider/googleoauth"
)

const (
	testKID      = "kid-google-test"
	testClientID = "test-client-id.apps.googleusercontent.com"
	testIssuer   = "https://accounts.google.com"
	testSubject  = "111122223333444455556"
)

// newJWKSServer serves a static JWKS containing the public half of key over
// loopback HTTP. NewProvider fetches its JWKS over HTTP by URL, so pointing it
// at an httptest.Server exercises the real fetch/parse/cache path with no
// external network and no faking of the keyfunc.
func newJWKSServer(t *testing.T, key *ecdsa.PrivateKey, kid string) string {
	t.Helper()

	jwk, err := jwkset.NewJWKFromKey(&key.PublicKey, jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{
			KID: kid,
			USE: jwkset.UseSig,
			ALG: jwkset.AlgES256,
		},
	})
	require.NoError(t, err)

	storage := jwkset.NewMemoryStorage()
	require.NoError(t, storage.KeyWrite(t.Context(), jwk))

	raw, err := storage.JSONPublic(t.Context())
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return key
}

// newVerifierConfig mirrors the shape every deployed app.config.yaml uses for
// oauth_providers.google.jwtverifier: the plural jwt_issuers allowlist and no
// singular jwt_issuer. Setting JWTIssuer here would make the issuer tests pass
// through a code path production never takes.
func newVerifierConfig(jwksURL string) config.JWTVerifierConfig {
	return config.JWTVerifierConfig{
		JWTIssuers:                []string{testIssuer, "accounts.google.com"},
		JWKSURL:                   jwksURL,
		JWKSRefreshInterval:       time.Hour,
		JWKSHTTPTimeout:           5 * time.Second,
		JWTLeeway:                 time.Second,
		JWKSUnknownKIDRefreshRate: time.Hour,
		JWKSUnknownKIDWaitMax:     time.Millisecond,
	}
}

// idTokenClaims mirrors the subset of Google's ID-token claims the provider
// reads, so tests can vary one field at a time.
type idTokenClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
	Name          string `json:"name,omitempty"`
	HostedDomain  string `json:"hd,omitempty"`
}

func newValidClaims(audience string) *idTokenClaims {
	now := time.Now()
	return &idTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testIssuer,
			Subject:   testSubject,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
		Email:         "alice@example.com",
		EmailVerified: true,
		Name:          "Alice",
	}
}

func signToken(t *testing.T, key *ecdsa.PrivateKey, kid string, claims jwt.Claims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(key)
	require.NoError(t, err)

	return signed
}

// newProvider wires a real Service against a loopback JWKS server and returns
// it together with the signing key the server publishes.
//
//nolint:unparam
func newProvider(ctx context.Context, t *testing.T, clientID string) (*googleoauth.Service, *ecdsa.PrivateKey) {
	t.Helper()

	key := newTestKey(t)
	srv, err := googleoauth.NewProvider(ctx, &config.GoogleOauthProvider{
		ClientID:  clientID,
		JWTVerify: newVerifierConfig(newJWKSServer(t, key, testKID)),
	})
	require.NoError(t, err)

	return srv, key
}

func TestNewProvider(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(t.Context(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("provider id is google", func(t *testing.T) {
		t.Parallel()

		srv, _ := newProvider(ctx, t, testClientID)
		require.Equal(t, entity.OAuthProviderGoogle, srv.ProviderID())
	})

	t.Run("construction tolerates an unreachable jwks endpoint", func(t *testing.T) {
		t.Parallel()

		// NewKeyFunc sets NoErrorReturnFirstHTTPReq, so a JWKS that is down at boot
		// must not stop the process from starting — verification fails later instead.
		srv, err := googleoauth.NewProvider(ctx, &config.GoogleOauthProvider{
			ClientID:  testClientID,
			JWTVerify: newVerifierConfig("http://127.0.0.1:1/jwks.json"),
		})
		require.NoError(t, err)
		require.NotNil(t, srv)
	})
}

func TestServiceVerifyToken(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(t.Context(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("accepts a well-formed token", func(t *testing.T) {
		t.Parallel()

		srv, key := newProvider(ctx, t, testClientID)

		claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, newValidClaims(testClientID)))
		require.NoError(t, err)
		require.Equal(t, testSubject, claims.Subject)
		require.Equal(t, "alice@example.com", claims.Email)
		require.Equal(t, "Alice", claims.Name)
	})

	// The security-critical property the clientID refactor touched: the audience
	// must be checked against this service's own client_id. A token minted by
	// Google for some *other* OAuth client is a perfectly valid Google signature,
	// so without this check any app's id_token would log a user in here.
	t.Run("rejects a token issued for a different audience", func(t *testing.T) {
		t.Parallel()

		srv, key := newProvider(ctx, t, testClientID)

		signed := signToken(t, key, testKID, newValidClaims("attacker-client-id.apps.googleusercontent.com"))

		claims, err := srv.VerifyToken(ctx, signed)
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
		require.ErrorIs(t, err, jwt.ErrTokenInvalidAudience)
	})

	t.Run("rejects a token with no audience at all", func(t *testing.T) {
		t.Parallel()

		srv, key := newProvider(ctx, t, testClientID)

		c := newValidClaims(testClientID)
		c.Audience = nil

		// jwt.WithAudience makes `aud` mandatory, so an absent audience surfaces as a
		// required-claim error rather than an audience mismatch.
		claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
		require.ErrorIs(t, err, jwt.ErrTokenRequiredClaimMissing)
	})

	t.Run("rejects a token signed by an unknown key", func(t *testing.T) {
		t.Parallel()

		srv, _ := newProvider(ctx, t, testClientID)

		// Correct kid, wrong key: the JWKS lookup succeeds but the signature does not.
		signed := signToken(t, newTestKey(t), testKID, newValidClaims(testClientID))

		claims, err := srv.VerifyToken(ctx, signed)
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("rejects an expired token", func(t *testing.T) {
		t.Parallel()

		srv, key := newProvider(ctx, t, testClientID)

		c := newValidClaims(testClientID)
		c.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
		c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))

		claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrTokenExpired)
	})

	t.Run("rejects a token from an unexpected issuer", func(t *testing.T) {
		t.Parallel()

		srv, key := newProvider(ctx, t, testClientID)

		c := newValidClaims(testClientID)
		c.Issuer = "https://evil.example.com"

		claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
		// Rejected by the jwt_issuers allowlist in validateClaims, not by the
		// parse-level jwt.WithIssuer check — deployed configs never set the
		// singular jwt_issuer that option needs, so it is not wired up. The
		// message names the allowlist, which is what pins the working check.
		require.ErrorContains(t, err, "unexpected issuer https://evil.example.com")
	})

	t.Run("rejects a token whose email is unverified", func(t *testing.T) {
		t.Parallel()

		srv, key := newProvider(ctx, t, testClientID)

		c := newValidClaims(testClientID)
		c.EmailVerified = false

		// Signature and registered claims are fine; validateClaims is what rejects
		// this, so the failure proves the post-parse claim validation runs.
		claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
		require.Contains(t, err.Error(), "email_verified")
	})

	t.Run("rejects a token with no expiration", func(t *testing.T) {
		t.Parallel()

		srv, key := newProvider(ctx, t, testClientID)

		c := newValidClaims(testClientID)
		c.ExpiresAt = nil

		claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("rejects an unsigned token", func(t *testing.T) {
		t.Parallel()

		srv, _ := newProvider(ctx, t, testClientID)

		token := jwt.NewWithClaims(jwt.SigningMethodNone, newValidClaims(testClientID))
		token.Header["kid"] = testKID
		signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)

		// alg=none must be refused by the WithValidMethods allow-list, and refused
		// *there* rather than incidentally downstream. Asserting the specific
		// signing-method error keeps this test honest: if "none" were ever added to
		// the allow-list, the JWKS alg binding would still reject the token, so a
		// bare ErrInvalidAccessToken assertion would keep passing and quietly stop
		// guarding the allow-list at all.
		claims, err := srv.VerifyToken(ctx, signed)
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
		require.ErrorIs(t, err, jwt.ErrTokenSignatureInvalid)
		require.Contains(t, err.Error(), "signing method none is invalid")
	})

	t.Run("rejects malformed input", func(t *testing.T) {
		t.Parallel()

		srv, _ := newProvider(ctx, t, testClientID)

		for _, token := range []string{"", "not-a-jwt", "a.b.c"} {
			claims, err := srv.VerifyToken(ctx, token)
			require.Nil(t, claims, "token %q", token)
			require.ErrorIs(t, err, apperr.ErrInvalidAccessToken, "token %q", token)
		}
	})

	t.Run("hosted domain restriction", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)
		cfg := newVerifierConfig(newJWKSServer(t, key, testKID))
		cfg.AllowedHostedDomains = []string{"example.com"}

		srv, err := googleoauth.NewProvider(ctx, &config.GoogleOauthProvider{
			ClientID:  testClientID,
			JWTVerify: cfg,
		})
		require.NoError(t, err)

		t.Run("accepts an allowed hd", func(t *testing.T) {
			c := newValidClaims(testClientID)
			c.HostedDomain = "example.com"

			claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, c))
			require.NoError(t, err)
			require.Equal(t, testSubject, claims.Subject)
		})

		t.Run("rejects a foreign hd", func(t *testing.T) {
			c := newValidClaims(testClientID)
			c.HostedDomain = "evil.com"

			claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, c))
			require.Nil(t, claims)
			require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
			require.Contains(t, err.Error(), "hd")
		})

		t.Run("rejects a missing hd", func(t *testing.T) {
			claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, newValidClaims(testClientID)))
			require.Nil(t, claims)
			require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
		})
	})

	t.Run("unknown kid against a served jwks", func(t *testing.T) {
		t.Parallel()

		srv, key := newProvider(ctx, t, testClientID)

		// The JWKS is reachable and non-empty, so authUnavailable() is false and the
		// error carries the underlying key-not-found cause rather than the
		// short-circuit message.
		claims, err := srv.VerifyToken(ctx, signToken(t, key, "kid-unknown", newValidClaims(testClientID)))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("jwks endpoint unreachable", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)
		srv, err := googleoauth.NewProvider(ctx, &config.GoogleOauthProvider{
			ClientID:  testClientID,
			JWTVerify: newVerifierConfig("http://127.0.0.1:1/jwks.json"),
		})
		require.NoError(t, err)

		// No keys were ever cached, so verification fails closed.
		claims, err := srv.VerifyToken(ctx, signToken(t, key, testKID, newValidClaims(testClientID)))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})
}

// TestJWKSServerServesParsableKeySet guards the fixture itself: if the served
// document stopped being a valid JWKS, every rejection test above would pass for
// the wrong reason.
func TestJWKSServerServesParsableKeySet(t *testing.T) {
	t.Parallel()

	url := newJWKSServer(t, newTestKey(t), testKID)

	resp, err := http.Get(url) //nolint:noctx // loopback fixture check
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var doc struct {
		Keys []struct {
			KID string `json:"kid"`
			KTY string `json:"kty"`
		} `json:"keys"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc))
	require.Len(t, doc.Keys, 1)
	require.Equal(t, testKID, doc.Keys[0].KID)
	require.Equal(t, "EC", doc.Keys[0].KTY)
}

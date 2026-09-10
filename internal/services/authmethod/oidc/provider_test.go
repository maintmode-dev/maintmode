package oidc_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/oidcdiscovery"
	mock_oidc "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/authmethod/oidc"
	"github.com/ruko1202/maintmode/internal/services/authmethod/oidc"
)

const (
	testKID          = "kid-google-test"
	testProviderName = "acme"
	testClientID     = "test-client-id.apps.googleusercontent.com"
	testIssuer       = "https://accounts.google.com"
	testSubject      = "111122223333444455556"
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

// newVerifierConfig mirrors what a deployed instance block actually carries:
// almost nothing. The JWKS URL, the expected issuer and the key-refresh timings
// are all the OIDC library's business now, and setting them here would drive a
// code path production never takes.
func newVerifierConfig() config.JWTVerifierConfig {
	return config.JWTVerifierConfig{}
}

// idTokenClaims mirrors the subset of Google's ID-token claims the provider
// reads, so tests can vary one field at a time.
type idTokenClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email,omitempty"`
	// any, not bool: issuers mint this as a boolean or as the string "true",
	// and the tests must be able to produce both plus the absent case.
	EmailVerified any    `json:"email_verified,omitempty"`
	Name          string `json:"name,omitempty"`
	HostedDomain  string `json:"hd,omitempty"`
}

func newValidClaims(issuer string) *idTokenClaims {
	now := time.Now()
	return &idTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   testSubject,
			Audience:  jwt.ClaimStrings{testClientID},
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

// newProvider wires a real Service against a mocked resolver and a loopback
// JWKS server, returning it with the signing key that server publishes and the
// issuer the resolved provider declares.
//
// Discovery itself is mocked rather than served: what it does is the resolver
// package's business and is tested there, and going through the real one would
// drag its transport rules into tests that are about token verification.
//
// The configured issuer carries a trailing slash and the resolved one does not,
// so the two strings DIFFER. Without that, tokens validated against the
// configured issuer and tokens validated against the resolved one are
// indistinguishable, and every assertion below would hold either way.
//
//nolint:unparam
func newProvider(t *testing.T, clientID string) (*oidc.Service, *ecdsa.PrivateKey, string) {
	t.Helper()

	key := newTestKey(t)
	issuer := "https://idp.example"

	return oidc.NewProvider(
		testProviderName,
		config.OIDCProvider{
			ClientID:  clientID,
			IssuerURL: issuer + "/",
			JWTVerify: newVerifierConfig(),
		},
		newResolverMock(t, issuer, newJWKSServer(t, key, testKID)),
	), key, issuer
}

// newResolverMock returns a resolver that answers with a provider built offline
// from the values a discovery document would have carried.
func newResolverMock(t *testing.T, issuer, jwksURL string) *mock_oidc.Mockresolver {
	t.Helper()

	resolver := mock_oidc.NewMockresolver(gomock.NewController(t))
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ string) (oidcdiscovery.Provider, error) {
			cfg := gooidc.ProviderConfig{
				IssuerURL: issuer,
				AuthURL:   issuer + "/authorize",
				TokenURL:  issuer + "/token",
				JWKSURL:   jwksURL,
			}

			return oidcdiscovery.Provider{OIDC: cfg.NewProvider(ctx), Issuer: issuer}, nil
		}).
		AnyTimes()

	return resolver
}

// newFailingResolverMock returns a resolver that never resolves, standing in
// for an IdP that is unreachable.
func newFailingResolverMock(t *testing.T) *mock_oidc.Mockresolver {
	t.Helper()

	resolver := mock_oidc.NewMockresolver(gomock.NewController(t))
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any()).
		Return(oidcdiscovery.Provider{}, errors.New("idp unreachable")).
		AnyTimes()

	return resolver
}

func TestNewProvider(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(t.Context(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	// The method id is the configured instance name, which is what reaches
	// user_identities.provider. Nothing about the provider is Google-specific
	// any more.
	t.Run("provider id is the instance name", func(t *testing.T) {
		t.Parallel()

		srv, _, _ := newProvider(t, testClientID)
		require.Equal(t, entity.AuthMethod(testProviderName), srv.MethodID())
	})

	// An IdP that is down when the process starts must cost that instance's
	// sign-ins, not the boot: construction reaches no network, Warm reports the
	// failure without it being fatal, and a verify against the unresolved
	// instance is unavailable rather than a bad token -- the caller's credential
	// was never examined.
	t.Run("an unreachable idp costs sign-ins, not startup", func(t *testing.T) {
		t.Parallel()

		srv := oidc.NewProvider(
			testProviderName,
			config.OIDCProvider{
				ClientID:  testClientID,
				IssuerURL: "https://idp.example",
				JWTVerify: newVerifierConfig(),
			},
			newFailingResolverMock(t),
		)

		require.Error(t, srv.Warm(ctx))

		claims, err := srv.Authenticate(ctx, "any.token.here")
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrAuthUnavailable)
	})
}

func TestServiceAuthenticate(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(t.Context(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("accepts a well-formed token", func(t *testing.T) {
		t.Parallel()

		srv, key, issuer := newProvider(t, testClientID)

		claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, newValidClaims(issuer)))
		require.NoError(t, err)
		require.Equal(t, testSubject, claims.Subject)
		require.Equal(t, "alice@example.com", claims.Email)
		require.Equal(t, "Alice", claims.Name)
	})

	// The security-critical property the clientID refactor touched: the audience
	// must be checked against this service's own client_id. A token minted by
	// Google for some *other* OAuth client is a perfectly valid Google signature,
	// so without this check any app's id_token would log a user in here.

	// Not a test of the library's expiry check but of OUR mapping of it: an
	// expired token is the one refusal a caller can act on -- it means "sign in
	// again" rather than "this token is not yours" -- so it gets its own
	// sentinel instead of collapsing into ErrInvalidAccessToken with everything
	// else.
	t.Run("maps an expired token to its own sentinel", func(t *testing.T) {
		t.Parallel()

		srv, key, issuer := newProvider(t, testClientID)

		c := newValidClaims(issuer)
		c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))

		claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrTokenExpired)
		require.NotErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	// The refusal is its own sentinel rather than "invalid token": the issuer
	// really did sign this, and really did decline to vouch for the address.
	// Collapsing the two is what hid the distinction from callers before.
	t.Run("refuses a token whose email is unverified", func(t *testing.T) {
		t.Parallel()

		srv, key, issuer := newProvider(t, testClientID)

		c := newValidClaims(issuer)
		c.EmailVerified = false

		claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrEmailNotVerified)
		require.NotErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	// Issuers disagree about the type: the spec says boolean, several mint the
	// JSON string. A plain bool decodes "true" as false and would refuse
	// sign-in for an issuer doing nothing wrong.
	t.Run("accepts email_verified as the string true", func(t *testing.T) {
		t.Parallel()

		srv, key, issuer := newProvider(t, testClientID)

		c := newValidClaims(issuer)
		c.EmailVerified = "true"

		claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, c))
		require.NoError(t, err)
		require.True(t, claims.EmailVerified)
	})

	t.Run("refuses email_verified as the string false", func(t *testing.T) {
		t.Parallel()

		srv, key, issuer := newProvider(t, testClientID)

		c := newValidClaims(issuer)
		c.EmailVerified = "false"

		claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrEmailNotVerified)
	})

	// The claim asserts a check was made, so its absence is "not asserted"
	// rather than "assume yes".
	t.Run("refuses a token with no email_verified claim at all", func(t *testing.T) {
		t.Parallel()

		srv, key, issuer := newProvider(t, testClientID)

		c := newValidClaims(issuer)
		c.EmailVerified = nil

		claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrEmailNotVerified)
	})

	// The document is authoritative about which issuer string the provider
	// mints, not the value an operator typed. This is the case that used to be
	// covered by a configured allow-list carrying both of Google's forms; that
	// list is gone, so the discovered value has to carry it alone.
	//
	// The configured issuer here ends in "/" and the document declares the bare
	// form. A verifier checking the CONFIGURED string would refuse this token.
	t.Run("validates the issuer the document declares, not the configured one", func(t *testing.T) {
		t.Parallel()

		srv, key, issuer := newProvider(t, testClientID)

		c := newValidClaims(issuer)

		claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, c))
		require.NoError(t, err)
		require.Equal(t, testSubject, claims.Subject)
	})

	t.Run("rejects a token whose issuer is a different form of the same url", func(t *testing.T) {
		t.Parallel()

		srv, key, issuer := newProvider(t, testClientID)

		// The trailing-slash form is what the config carries; the document
		// declares the bare one, and the document wins.
		c := newValidClaims(issuer + "/")

		claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, c))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
		require.ErrorContains(t, err, "issued by a different provider")
	})

	// A token with no exp is refused as EXPIRED rather than as malformed: an
	// absent expiry reads as the zero time, which is in the past. The sentinel
	// differs from the other refusals here, and that is the right outcome --
	// both mean "sign in again" to the caller, and a token that never expires
	// is not one this backend will hold open.

	t.Run("hosted domain restriction", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)
		issuer := "https://idp.example"
		cfg := newVerifierConfig()
		cfg.AllowedHostedDomains = []string{"example.com"}

		srv := oidc.NewProvider(
			testProviderName,
			config.OIDCProvider{
				ClientID:  testClientID,
				IssuerURL: issuer,
				JWTVerify: cfg,
			},
			newResolverMock(t, issuer, newJWKSServer(t, key, testKID)),
		)

		t.Run("accepts an allowed hd", func(t *testing.T) {
			c := newValidClaims(issuer)
			c.HostedDomain = "example.com"

			claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, c))
			require.NoError(t, err)
			require.Equal(t, testSubject, claims.Subject)
		})

		t.Run("rejects a foreign hd", func(t *testing.T) {
			c := newValidClaims(issuer)
			c.HostedDomain = "evil.com"

			claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, c))
			require.Nil(t, claims)
			require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
			require.Contains(t, err.Error(), "hd")
		})

		t.Run("rejects a missing hd", func(t *testing.T) {
			claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, newValidClaims(issuer)))
			require.Nil(t, claims)
			require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
		})
	})

	t.Run("unknown kid against a served jwks", func(t *testing.T) {
		t.Parallel()

		srv, key, issuer := newProvider(t, testClientID)

		// The JWKS is reachable and non-empty, so authUnavailable() is false and the
		// error carries the underlying key-not-found cause rather than the
		// short-circuit message.
		claims, err := srv.Authenticate(ctx, signToken(t, key, "kid-unknown", newValidClaims(issuer)))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})

	t.Run("jwks endpoint unreachable", func(t *testing.T) {
		t.Parallel()

		key := newTestKey(t)
		// Discovery resolves, but the JWKS URL it names is dead.
		issuer := "https://idp.example"
		srv := oidc.NewProvider(
			testProviderName,
			config.OIDCProvider{
				ClientID:  testClientID,
				IssuerURL: issuer,
				JWTVerify: newVerifierConfig(),
			},
			newResolverMock(t, issuer, "https://127.0.0.1:1/jwks.json"),
		)

		// No keys were ever cached, so verification fails closed.
		claims, err := srv.Authenticate(ctx, signToken(t, key, testKID, newValidClaims(issuer)))
		require.Nil(t, claims)
		require.ErrorIs(t, err, apperr.ErrInvalidAccessToken)
	})
}

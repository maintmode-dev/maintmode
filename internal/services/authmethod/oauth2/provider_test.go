package oauth2_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	mock_oauth2 "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/services/authmethod/oauth2"
	"github.com/ruko1202/maintmode/internal/services/authmethod/oauth2"
)

// newGateway builds the mocked identity half.
//
// The gateway's own wire behavior -- headers, paths, status handling, the
// email-selection rule -- is covered against a real httptest server in
// gateways/github. What is left for this layer is the mapping, so the gateway is
// mocked rather than served.
func newGateway(t *testing.T) *mock_oauth2.MockidentityGateway {
	t.Helper()

	return mock_oauth2.NewMockidentityGateway(gomock.NewController(t))
}

// TestMethodIDIsTheInstanceName is the rule, and it replaces the opposite one.
//
// An earlier version returned a per-vendor CONSTANT here, on the reasoning that
// the registry is keyed by method while user_identities.provider stores the
// instance name. Those are not two values: NewAuthMethods keys its map by
// item.MethodID(), and a dance callback looks an instance up by the name in its
// URL path. So MethodID IS the registry key, the path segment and the stored
// provider string, all at once.
//
// A constant therefore collapsed every instance of one vendor onto a single key:
// configure github.com and a GitHub Enterprise Server side by side, and whichever
// was constructed last silently replaced the other -- no error, no log line, and
// a sign-in button that authenticated against the wrong host. Returning the
// instance name is what the OIDC provider has always done, and it is why google,
// okta and keycloak coexist.
//
// An instance named "github" still writes exactly the string it always wrote,
// which is what keeps existing user_identities rows matching.
func TestMethodIDIsTheInstanceName(t *testing.T) {
	t.Parallel()

	provider := oauth2.NewProvider("github-enterprise", newGateway(t))

	assert.Equal(t, entity.AuthMethod("github-enterprise"), provider.MethodID(),
		"two instances of one vendor must not collapse onto a single registry key")
}

// TestAuthenticateMapsClaims is the whole contract of this layer.
//
// EmailVerified is the assertion that matters, and it is not cosmetic. The field
// is read by services/invitation/accept.go, which refuses with ErrEmailMismatch
// when it is false -- so a provider leaving it at the zero value would fail
// EVERY invited GitHub sign-in with "that address does not match", on an address
// that matches perfectly. It is set true here because the gateway already
// enforced primary && verified one layer down; this is the same check reported,
// not a second one assumed.
func TestAuthenticateMapsClaims(t *testing.T) {
	t.Parallel()

	gateway := newGateway(t)
	// The credential reaches the gateway unchanged: it is the access token, and
	// this layer adds nothing to it.
	gateway.EXPECT().
		FetchIdentity(gomock.Any(), "gho_token").
		Return(&entity.OAuth2Identity{
			Subject: "4242",
			Email:   "octocat@example.com",
			Name:    "The Octocat",
		}, nil)

	claims, err := oauth2.NewProvider("github", gateway).Authenticate(t.Context(), "gho_token")
	require.NoError(t, err)

	assert.Equal(t, "4242", claims.Subject)
	assert.Equal(t, "octocat@example.com", claims.Email)
	assert.Equal(t, "The Octocat", claims.Name)
	assert.True(t, claims.EmailVerified,
		"a zero EmailVerified reads as 'the issuer reports this address unverified' and breaks every invited sign-in")
}

// TestAuthenticatePropagatesGatewayFailures pins that this layer adds no
// interpretation of its own.
//
// The gateway's sentinels have to survive: the dance path tells an unusable
// email apart from a provider outage by matching on them, and a wrap that lost
// errors.Is would collapse the two into one audit reason.
func TestAuthenticatePropagatesGatewayFailures(t *testing.T) {
	t.Parallel()

	for _, sentinel := range []error{
		apperr.ErrGithubEmailUnusable,
		apperr.ErrGithubIdentityUnusable,
		apperr.ErrAuthUnavailable,
	} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			t.Parallel()

			gateway := newGateway(t)
			gateway.EXPECT().FetchIdentity(gomock.Any(), "tok").Return(nil, sentinel)

			_, err := oauth2.NewProvider("github", gateway).Authenticate(t.Context(), "tok")

			require.Error(t, err)
			assert.True(t, errors.Is(err, sentinel), "the sentinel must survive the wrap")
		})
	}
}

// TestAuthenticateRefusesEmptyCredential saves a guaranteed-to-fail outbound
// request.
//
// An empty access token can only come from a bug on our side -- Exchange refuses
// to return one -- so it is refused here rather than spent on api.github.com.
// The mock asserts the negative: with no EXPECT registered, any call fails the
// test.
func TestAuthenticateRefusesEmptyCredential(t *testing.T) {
	t.Parallel()

	gateway := newGateway(t)

	_, err := oauth2.NewProvider("github", gateway).Authenticate(t.Context(), "")

	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidCredentials)
}

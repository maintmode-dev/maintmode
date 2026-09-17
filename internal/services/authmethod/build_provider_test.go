package authmethod

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	oauth2gw "github.com/ruko1202/maintmode/internal/gateways/oauth2"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	oauth2method "github.com/ruko1202/maintmode/internal/services/authmethod/oauth2"
)

// Build carries five fields from the stored settings into two different
// objects. Each is a cross-layer assignment the compiler cannot check: swap two
// of them and everything still builds, while sign-in fails in a way that points
// nowhere near this function.
func TestReloader_BuildProvider(t *testing.T) {
	t.Parallel()

	t.Run("refuses settings of another kind", func(t *testing.T) {
		t.Parallel()

		got := newTestReloader().buildOne(context.Background(), "keycloak",
			entity.ConfiguredProvider{
				Name:     "keycloak",
				Enabled:  true,
				Settings: integrationkinds.SlackSettings{},
			})

		require.Equal(t, entity.LoginProviderHealthUnreadable, got.Health,
			"a type mismatch disables that provider, not the rebuild")
		require.Nil(t, got.Method, "nothing usable may be built from the wrong shape")
		require.Nil(t, got.Gateway)
	})

	// An unreachable issuer builds fine, and that is the point: a rebuild reads
	// the stored row and nothing else, so one IdP being down cannot stall or
	// fail it. The verifier resolves on first use instead -- the cost moves to
	// the first sign-in through that provider, and to nobody else.
	//
	// 127.0.0.1:1 is chosen because it cannot answer: if anything here reached
	// for discovery, this test would fail rather than pass slowly.
	t.Run("builds without reaching the issuer", func(t *testing.T) {
		t.Parallel()

		built := newTestReloader().buildOne(context.Background(), "unreachable",
			entity.ConfiguredProvider{
				Name:    "unreachable",
				Enabled: true,
				Settings: integrationkinds.OIDCSettings{
					IssuerURL: "https://127.0.0.1:1/nowhere",
					ClientID:  "client",
				},
			})

		require.Equal(t, entity.LoginProviderHealthOK, built.Health,
			"building must not depend on the IdP answering")
		require.NotNil(t, built.Method)
		require.NotNil(t, built.Gateway)
	})

	// The second login SHAPE -- plain OAuth 2.0, not one vendor -- and the
	// reason buildOne switches on the settings type rather than asking "is this
	// OIDC".
	//
	// Asserting the concrete types is the whole point. A test that only checked
	// "something was built" passes against the arm being deleted entirely --
	// verified by mutation: removing the GitHub case left every other test in
	// this package green, because a row that matches no arm is a per-provider
	// failure and no other test configures one.
	t.Run("builds an oauth2 provider from oauth2 settings", func(t *testing.T) {
		t.Parallel()

		built := newTestReloader().buildOne(context.Background(), "github",
			entity.ConfiguredProvider{
				Name:    "github",
				Enabled: true,
				Settings: integrationkinds.OAuth2Settings{
					DisplayName:  "GitHub",
					ClientID:     "Iv1.client",
					ClientSecret: "secret",
					RedirectURI:  "https://example.com/auth/api/v1/login/oauth/github/callback",
					AuthorizeURL: "https://github.example/login/oauth/authorize",
					TokenURL:     "https://github.example/login/oauth/access_token",
					APIBaseURL:   "https://api.github.example",
				},
			})

		require.Equal(t, entity.LoginProviderHealthOK, built.Health)
		require.IsType(t, (*oauth2method.Service)(nil), built.Method,
			"a github row must not resolve to an OIDC provider built from a zero value")
		require.IsType(t, (*oauth2gw.Client)(nil), built.Gateway)
		require.Equal(t, "GitHub", built.DisplayName)
		require.Equal(t,
			"https://example.com/auth/api/v1/login/oauth/github/callback", built.RedirectURI)

		// Seven fields cross from the stored row into the gateway, and the
		// compiler checks none of them: they are all strings. The endpoints are
		// the ones worth asserting through behavior rather than by reading the
		// struct back -- dropping them builds a client that silently talks to
		// nowhere, and AuthCodeURL is where that becomes visible.
		start, err := built.Gateway.AuthCodeURL(context.Background(), "state", "verifier")
		require.NoError(t, err)
		require.Contains(t, start, "https://github.example/login/oauth/authorize",
			"the authorize endpoint must come from the stored row")
		require.Contains(t, start, "client_id=Iv1.client")
	})

	// The fallback lives in buildOne and keys on the ROW NAME, so it holds for
	// every shape without a settings type doing anything to earn it.
	t.Run("a blank display name falls back to the instance name", func(t *testing.T) {
		t.Parallel()

		built := newTestReloader().buildOne(context.Background(), "github",
			entity.ConfiguredProvider{
				Name:     "github",
				Enabled:  true,
				Settings: integrationkinds.OAuth2Settings{ClientID: "id"},
			})

		require.Equal(t, "github", built.DisplayName, "a button is never blank")
	})
}

// newTestReloader builds one with nothing wired but the resolver, which is all
// buildOne touches.
func newTestReloader() *Reloader {
	return NewReloader(nil, nil, testResolver())
}

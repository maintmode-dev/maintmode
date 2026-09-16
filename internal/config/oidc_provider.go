package config

import (
	"fmt"
	"slices"
)

// reservedInstanceKeys are names an instance may not take.
//
// They name mechanisms rather than providers: stub verifies nothing and is
// substituted for every method on a use_stub stand; bootstrap resolves its
// identity by configured email and skips the seats cap; unknown is an
// output-only sentinel. The last two are the ids GET /auth/providers already
// emits, which an instance would collide with in that response.
//
// google, github and email are deliberately absent. google must be usable —
// existing identities depend on the string — and the other two are held for the
// follow-up work that implements them, which would otherwise inherit a name it
// cannot use.
var reservedInstanceKeys = []string{
	"stub",
	"bootstrap",
	"unknown",
	"email_password",
	"email_otp",
}

// OIDCProvider configures one OIDC identity provider.
//
// Instances differ only by issuer and credentials: the discovery document at
// the issuer supplies the endpoints and the JWKS location, so Google, Keycloak,
// Okta and Authentik all run the same code. There is deliberately no endpoint
// override — see internal/gateways/oidcdiscovery.
type OIDCProvider struct {
	// DisplayName labels the sign-in button. It is the supported way to
	// relabel a provider; the map key is not.
	DisplayName string `mapstructure:"display_name"`
	// IssuerURL identifies the provider and is the base for its discovery
	// document. It is compared against the issuer the document declares.
	IssuerURL string `mapstructure:"issuer_url"`
	ClientID  string `mapstructure:"client_id"`
	// ClientSecret is the confidential-client credential used at the token
	// endpoint. Resolved from the secret store via a <secret:...> reference —
	// never written literally into a config file.
	ClientSecret string `mapstructure:"client_secret"`
	// RedirectURI is this instance's EXTERNAL callback URL, and it must be the
	// external form. Caddy serves the backend under `handle_path /auth/*`,
	// which strips the prefix, so the app sees /api/v1/... while the browser
	// and the provider see /auth/api/v1/... Registering the internal form
	// yields a redirect_uri_mismatch that never reaches our logs.
	RedirectURI string `mapstructure:"redirect_uri"`
	// Scopes overrides the requested scopes for providers that gate claims
	// behind an extra one. Empty means the default set.
	Scopes    []string          `mapstructure:"scopes"`
	JWTVerify JWTVerifierConfig `mapstructure:"jwtverifier"`
}

// OauthProviders has no `stub` section: the stub short-circuits verification in
// dev and reads nothing from config, so there is no StubOauthProvider type.
// UseStub (gated on IsDev) is the only stub-related knob.
type OauthProviders struct {
	UseStub bool `mapstructure:"use_stub"`
	// Presets is the credential-free catalog of well-known login providers.
	// It configures nothing by itself -- see LoginPresets.
	Presets LoginPresets `mapstructure:"presets"`
}

// ValidateInstanceKey reports whether key may name an OIDC instance.
//
// Only reserved names are refused; the character set is not policed. It used to
// be, on three arguments that did not survive checking: the column is free TEXT
// written through a parameterised query, the route segment is matched against
// the registry rather than pattern-routed, and the dance state signature became
// injective on its own once its MAC fields were length-prefixed. A rule with no
// remaining reason is worse than none -- the next reader inherits it as a
// constraint to preserve.
func ValidateInstanceKey(key string) error {
	if key == "" {
		return fmt.Errorf("oauth provider instance name must not be empty")
	}

	if slices.Contains(reservedInstanceKeys, key) {
		return fmt.Errorf("oauth provider instance name %q is reserved", key)
	}

	return nil
}

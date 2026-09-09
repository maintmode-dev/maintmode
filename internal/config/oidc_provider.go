package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
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
	// OIDC holds the configured providers keyed by instance name. The key is
	// identity, and renaming one orphans every linked
	// account on it.
	OIDC map[string]OIDCProvider `mapstructure:"oidc"`
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

// DanceCookieSecure reports whether the OAuth dance cookies should carry the
// Secure attribute.
//
// Secure unless every instance that can dance has a plain-http redirect URI.
// The asymmetry is the point: a production cookie must never lose Secure
// because some other instance is plain, whereas the reverse mistake -- an http
// instance alongside an https one getting Secure -- costs a local sign-in, and
// only outside prod where http is configured at all.
//
// Anything unreadable counts as https, matching the rest of this rule's
// fail-safe direction: a cookie the browser withholds beats one it leaks. So
// does having no dance-capable instance at all.
func (o OauthProviders) DanceCookieSecure() bool {
	danceable := 0

	for _, provider := range o.OIDC {
		if !provider.DanceConfigured() {
			continue
		}

		danceable++

		parsed, err := url.Parse(provider.RedirectURI)
		if err != nil || parsed.Scheme != "http" {
			return true
		}
	}

	return danceable == 0
}

// DanceConfigured reports whether this instance can run the backend-driven
// dance, which needs the confidential-client half: a secret to authenticate at
// the token endpoint and a callback URL to come back to.
//
// Separate from validate because an instance is legitimately useful without
// them. The BFF path verifies ID tokens the frontend obtained itself and needs
// only client_id, which is how every stand ships today.
func (p OIDCProvider) DanceConfigured() bool {
	return strings.TrimSpace(p.ClientSecret) != "" && strings.TrimSpace(p.RedirectURI) != ""
}

// validate checks the one thing about an instance that nothing downstream can.
//
// Almost nothing else needs checking here, which is why almost nothing else is:
// an empty or malformed issuer_url is refused by discovery, an empty client_id
// is refused by the verifier on every token ("clientID must be provided"), and
// an empty display_name is a button with no label. Restating those as startup
// rules buys an earlier message for failures that are already loud, at the cost
// of a second place to keep in step with the library.
//
// The dance credentials are the exception. They are all-or-nothing: an instance
// with a client_secret but no redirect_uri registers its routes and then sends
// the browser somewhere it cannot come back from -- a 302 that reads as success
// in every access log. Neither half alone is visible anywhere downstream, so
// this is the check that has to live here.
//
// Expressed as two mirrored rules rather than one comparison because that is
// what names the missing half in the error: "redirect_uri: cannot be blank"
// rather than a sentence about both.
func (p OIDCProvider) validate(key string) error {
	err := validation.ValidateStruct(&p,
		validation.Field(&p.ClientSecret, validation.Required.When(strings.TrimSpace(p.RedirectURI) != "")),
		validation.Field(&p.RedirectURI, validation.Required.When(strings.TrimSpace(p.ClientSecret) != "")),
	)
	if err != nil {
		return fmt.Errorf("oauth_providers.oidc.%s: %w", key, err)
	}

	return nil
}

// DanceInstanceNames returns, in a stable order, the instances that can run the
// backend dance.
func (o OauthProviders) DanceInstanceNames() []string {
	names := make([]string, 0, len(o.OIDC))
	for _, name := range o.InstanceNames() {
		if o.OIDC[name].DanceConfigured() {
			names = append(names, name)
		}
	}

	return names
}

// InstanceNames returns the configured instance names in a stable order.
//
// Sorted because map iteration is randomized, and these names reach the
// sign-in screen: an unsorted list would reshuffle the buttons between requests
// and between replicas.
func (o OauthProviders) InstanceNames() []string {
	names := make([]string, 0, len(o.OIDC))
	for name := range o.OIDC {
		names = append(names, name)
	}
	slices.Sort(names)

	return names
}

// validateOIDCProviders checks every configured instance at startup.
func (c *AppConfig) validateOIDCProviders() error {
	for _, key := range c.OauthProviders.InstanceNames() {
		if err := ValidateInstanceKey(key); err != nil {
			return err
		}

		if err := c.OauthProviders.OIDC[key].validate(key); err != nil {
			return err
		}
	}

	return nil
}

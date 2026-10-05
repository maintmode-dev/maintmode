package config

// LoginFacts is what is known about a sign-in provider regardless of who
// operates it: the parts an operator should not have to look up, and could get
// subtly wrong. The deployment config ships them for the well-known providers,
// the way Grafana's defaults.ini ships [auth.github]'s endpoints.
type LoginFacts struct {
	// DisplayName is the label on the sign-in button.
	DisplayName string `mapstructure:"display_name"`
	// IssuerURL is the OIDC issuer. It is also an input to the AAD of the
	// provider's client_secret, which is why an API-created provider gets it
	// COPIED INTO THE ROW at create rather than read from here at use time --
	// see the package comment on services/integration/secrets.go. Reading it live
	// would mean one edit to this file changes the AAD of every secret sealed
	// under it, and every affected provider fails at the next sign-in.
	IssuerURL string `mapstructure:"issuer_url"`
	// AuthorizeURL, TokenURL and APIBaseURL are the endpoints of a provider
	// that publishes no discovery document, so they cannot be fetched the way an
	// OIDC issuer's are. They are the same KIND of fact as IssuerURL -- an
	// address of the outside world an operator should not have to look up -- and
	// live here for the same reason: moving a vendor to a different host, or
	// pointing a stand at a fake one, is then a config edit rather than a
	// release.
	//
	// Empty for an OIDC provider, which has no use for them. Fields omits what
	// is empty, so an unset field is neither written into a row nor compared
	// against one.
	AuthorizeURL string `mapstructure:"authorize_url"`
	TokenURL     string `mapstructure:"token_url"`
	// APIBaseURL is the API ROOT, and sub-resources are joined onto it as paths.
	// Grafana's equivalent expects a URL already pointing at /user and
	// concatenates "/emails" onto it, which is a documented source of
	// misconfiguration; this is the other choice deliberately.
	APIBaseURL string `mapstructure:"api_base_url"`
}

// Fields is the facts as the config keys they own, mapped to their values.
//
// One place naming those keys, deliberately. The integration service both
// WRITES them (at create) and COMPARES them (at update), and those are separate
// functions for a reason -- injecting on update would silently repair a
// tampered field instead of refusing it. But if each kept its own list of key
// names, adding a field would have to be remembered in two places, and
// forgetting it in the comparison is exactly the account-takeover hole the
// comparison exists to close.
//
// Empty values are OMITTED rather than fixed to "". An OIDC entry has no
// authorize_url, and writing an empty one into the row would both fail that
// kind's validation and make the comparison demand a field the settings type
// does not carry.
func (f LoginFacts) Fields() map[string]string {
	candidates := map[string]string{
		"issuer_url":    f.IssuerURL,
		"display_name":  f.DisplayName,
		"authorize_url": f.AuthorizeURL,
		"token_url":     f.TokenURL,
		"api_base_url":  f.APIBaseURL,
	}

	fields := make(map[string]string, len(candidates))
	for key, value := range candidates {
		if value != "" {
			fields[key] = value
		}
	}

	return fields
}

// LoginProvider is one provider's section, flat like Grafana's [auth.github]:
// the facts, `enabled`, and every other setting side by side.
//
// Who owns it is ManagedEntry's managed_by. Under ui only the facts are read --
// they are copied into the provider at create and cannot be changed through
// the API. Either way its facts are what an admin-created provider of that
// name takes, including one handed back to the UI by switching config to ui.
type LoginProvider struct {
	LoginFacts   `mapstructure:",squash"`
	ManagedEntry `mapstructure:",squash"`
}

// LoginProviders is every provider section, keyed by the registry's system
// name.
type LoginProviders map[string]LoginProvider

// For returns the entry filed under a key, and whether one exists.
//
// For a preset-backed provider the second return value must be honored as a
// REFUSAL, never as "this provider has no defaults". Treating a missing entry
// as "the operator supplies everything" would let someone delete one block of
// YAML and then create `google` through the API pointed at an IdP they control
// -- which mints whatever subject it likes against the user_identities rows
// already bound to it. `custom` is the one name that legitimately has no
// entry, and it is legitimate because the name itself says so.
func (p LoginProviders) For(key string) (LoginProvider, bool) {
	provider, ok := p[key]

	return provider, ok
}

// prepareProviders checks every provider section's mode and moves its
// <secret:KEY> references from Settings into Secrets. See prepareEntries.
func (p OauthProviders) prepareProviders() error {
	return prepareEntries("oauth_providers.providers", p.Providers,
		func(provider *LoginProvider) *ManagedEntry { return &provider.ManagedEntry })
}

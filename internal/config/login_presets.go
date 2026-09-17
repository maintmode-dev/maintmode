package config

// LoginPreset is what this product already knows about a well-known sign-in
// provider: the parts an operator should not have to look up, and could get
// subtly wrong.
//
// It carries NO credential. A preset is reference data about the outside world
// -- Google's issuer URL has not moved in a decade -- while the client id and
// secret belong to the operator's own OAuth client and live encrypted in the
// registry. That split is what lets this stay in a file: editing it is a
// deploy-time act with nothing sensitive in it.
type LoginPreset struct {
	// DisplayName is the label on the sign-in button.
	DisplayName string `mapstructure:"display_name"`
	// IssuerURL is the OIDC issuer. It is also an input to the AAD of the
	// provider's client_secret, which is why it is COPIED INTO THE ROW at create
	// rather than read from here at use time -- see the package comment on
	// services/integration/secrets.go. Reading it live would mean one edit to
	// this file changes the AAD of every secret sealed under it, and every
	// affected provider fails at the next sign-in.
	IssuerURL string `mapstructure:"issuer_url"`
	// AuthorizeURL, TokenURL and APIBaseURL are the endpoints of a provider
	// that publishes no discovery document, so they cannot be fetched the way an
	// OIDC issuer's are. They are the same KIND of fact as IssuerURL -- an
	// address of the outside world an operator should not have to look up -- and
	// live here for the same reason: moving a vendor to a different host, or
	// pointing a stand at a fake one, is then a config edit rather than a
	// release.
	//
	// Empty for an OIDC preset, which has no use for them. Fields omits what is
	// empty, so an unset field is neither written into a row nor compared
	// against one.
	AuthorizeURL string `mapstructure:"authorize_url"`
	TokenURL     string `mapstructure:"token_url"`
	// APIBaseURL is the API ROOT, and sub-resources are joined onto it as paths.
	// Grafana's equivalent expects a URL already pointing at /user and
	// concatenates "/emails" onto it, which is a documented source of
	// misconfiguration; this is the other choice deliberately.
	APIBaseURL string `mapstructure:"api_base_url"`
}

// LoginPresets is the catalog, keyed by the registry's system name.
//
// It is NOT a provider list. An entry creates nothing, enables nothing and
// holds no credential; a provider exists only when a row exists. Adding a
// sixth entry here does not produce a sixth provider -- the registry refuses
// the pair -- which is what keeps this from becoming a second source of truth
// beside the registry.
type LoginPresets map[string]LoginPreset

// Fields is the preset as the config keys it owns, mapped to the values it
// fixes them to.
//
// One place naming those keys, deliberately. The integration service both
// WRITES them (at create) and COMPARES them (at update), and those are separate
// functions for a reason -- injecting on update would silently repair a
// tampered field instead of refusing it. But if each kept its own list of key
// names, adding a third preset field would have to be remembered in two places,
// and forgetting it in the comparison is exactly the account-takeover hole the
// comparison exists to close.
// Empty values are OMITTED rather than fixed to "". A preset fixes only the
// fields it actually knows: an OIDC entry has no authorize_url, and writing an
// empty one into the row would both fail that kind's validation and make the
// comparison demand a field the settings type does not carry.
func (p LoginPreset) Fields() map[string]string {
	candidates := map[string]string{
		"issuer_url":    p.IssuerURL,
		"display_name":  p.DisplayName,
		"authorize_url": p.AuthorizeURL,
		"token_url":     p.TokenURL,
		"api_base_url":  p.APIBaseURL,
	}

	fields := make(map[string]string, len(candidates))
	for key, value := range candidates {
		if value != "" {
			fields[key] = value
		}
	}

	return fields
}

// For returns the preset for a system name, and whether one exists.
//
// The second return value must be honored as a REFUSAL, never as "this
// provider has no defaults". Treating a missing entry as "the operator supplies
// everything" would let someone delete one line of YAML and then create
// `google` pointed at an IdP they control -- which mints whatever subject it
// likes against the user_identities rows already carrying provider='google'.
// `custom` is the one name that legitimately has no entry, and it is legitimate
// because the name itself says so.
func (p LoginPresets) For(name string) (LoginPreset, bool) {
	preset, ok := p[name]

	return preset, ok
}

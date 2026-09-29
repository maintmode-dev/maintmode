package config

import (
	"errors"
	"fmt"
)

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
// ManagedBy says who owns the provider, and is required -- there is no default,
// so no entry falls into a mode by omission:
//
//   - config: the provider is declared HERE. The integration service writes it
//     into the registry at startup, serves it from memory, and refuses every
//     admin write to it. Enabled is required; credentials only when it is true;
//   - ui: an admin creates and runs the provider in the UI. Only the facts are
//     read -- everything else in the entry is ignored -- and they are copied
//     into the provider at create and cannot be changed through the API.
//
// Either way its facts are what an admin-created provider of that name takes,
// including one handed back to the UI by switching config to ui.
type LoginProvider struct {
	LoginFacts `mapstructure:",squash"`
	// ManagedBy is ManagedByConfig or ManagedByUI.
	ManagedBy string `mapstructure:"managed_by"`
	// Enabled is on/off, and nothing else. Required under config, ignored
	// under ui, where the UI switches the provider.
	Enabled *bool `mapstructure:"enabled"`
	// Settings is every other key, flat: client_id, redirect_uri, scopes, ...
	// After prepareProviders it holds no <secret:KEY> reference.
	Settings map[string]any `mapstructure:",remain"`
	// Secrets is the keys whose value is a <secret:KEY> reference, moved out of
	// Settings at load so the secret resolver -- which rewrites
	// map[string]string values -- resolves them. Never decoded from the file
	// directly.
	Secrets map[string]string `mapstructure:"-"`
}

// The two owners of a provider.
const (
	ManagedByConfig = "config"
	ManagedByUI     = "ui"
)

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
// <secret:KEY> references from Settings into Secrets. It runs BEFORE the
// secrets are applied: telling a reference from a literal is only possible
// while the reference is still there, and the resolver only reaches Secrets.
func (p OauthProviders) prepareProviders() error {
	var errs error
	for name, provider := range p.Providers {
		field := "oauth_providers.providers." + name

		if err := checkMode(provider); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", field, err))
			continue
		}

		// Under ui everything but the facts is ignored -- the UI supplies it --
		// secrets included: a leftover <secret:...> must not fail startup over
		// a key the secrets file no longer carries.
		if provider.ManagedBy != ManagedByConfig {
			continue
		}

		moveSecrets(&provider)

		p.Providers[name] = provider
	}

	return errs
}

// moveSecrets moves every key whose value is a <secret:KEY> reference from
// Settings into Secrets. What is a secret is what the file marks as one: the
// config cannot know which fields a provider treats as secret -- the provider's
// kind does, and the integration service holds the entry to it.
//
// The move is needed because the secret resolver rewrites map[string]string
// values but cannot reach a value inside map[string]any: a reference left in
// Settings would stay the literal text "<secret:...>".
func moveSecrets(provider *LoginProvider) {
	for key, value := range provider.Settings {
		ref, ok := value.(string)
		if !ok || !secretRegexp.MatchString(ref) {
			continue
		}

		if provider.Secrets == nil {
			provider.Secrets = map[string]string{}
		}
		provider.Secrets[key] = ref
		delete(provider.Settings, key)
	}
}

// checkMode holds an entry to the rules of its managed_by. A missing or
// misspelled mode fails startup rather than quietly deciding who owns a
// provider.
func checkMode(provider LoginProvider) error {
	switch provider.ManagedBy {
	case ManagedByConfig:
		if provider.Enabled == nil {
			return errors.New("enabled must be set when managed_by is config")
		}

		return nil

	case ManagedByUI:
		return nil

	default:
		return fmt.Errorf("managed_by must be config or ui, got %q", provider.ManagedBy)
	}
}

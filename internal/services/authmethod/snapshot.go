package authmethod

import (
	"context"
	"net/url"
	"sort"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Gateway is the confidential half of one provider: the calls that need the
// client secret.
//
// It is declared here, rather than imported from services/auth where the
// equivalent interface already lives, because auth imports this package and the
// reverse would close a cycle. The method set is identical, so the same
// *oidc.Client satisfies both structurally and nothing has to move.
type Gateway interface {
	AuthCodeURL(ctx context.Context, state, verifier string) (string, error)
	Exchange(ctx context.Context, code, codeVerifier string) (string, error)
}

// snapshot is the whole login configuration at one instant.
//
// It is replaced wholesale, never mutated. A reader Loads the pointer once and
// uses that value to completion, so a reload cannot tear: a request finishes
// against the providers it started with, and a caller can never see a button
// whose gateway is missing.
//
// Every half lives in ONE snapshot for exactly that reason. Split across
// separate pointers they could skew, and a button that leads to an error is
// worse than no button.
type snapshot struct {
	// methods verify an asserted identity; no client secret involved.
	methods map[entity.AuthMethod]AuthMethod
	// gateways hold the confidential half, and their KEYS are the set that can
	// run the backend-driven flow -- having the gateway is what danceable
	// means. A separate set beside them would be the same keys written twice,
	// with a state ("danceable but no gateway") that cannot be built and would
	// mean a button leading nowhere.
	gateways map[entity.AuthMethod]Gateway
	// listing is what /auth/providers renders, pre-sorted.
	listing []entity.LoginMethodView
	// health is admin-facing state, keyed by provider.
	health map[entity.AuthMethod]entity.LoginProviderHealth
	// danceCookieSecure is whether the dance cookies carry Secure.
	//
	// Aggregated over the providers that can dance, and it leans deliberately:
	// Secure unless EVERY such provider redirects over plain http, with no
	// dance-capable provider at all also counting as secure. A production cookie
	// must never lose Secure because some other provider is plain, whereas the
	// reverse mistake costs a local sign-in -- and only where http is configured
	// at all.
	danceCookieSecure bool
}

// NewSnapshot assembles a snapshot from the providers plus the built-in methods
// the caller always wants present.
//
// builtins are the non-provider methods (password, email code, the dev stub and
// bootstrap): they are not configured through the registry and must survive
// every reload untouched.
func newSnapshot(builtins map[entity.AuthMethod]AuthMethod, providers []providerInput) *snapshot {
	methods := make(map[entity.AuthMethod]AuthMethod, len(builtins)+len(providers))
	for id, m := range builtins {
		methods[id] = m
	}

	gateways := make(map[entity.AuthMethod]Gateway, len(providers))
	health := make(map[entity.AuthMethod]entity.LoginProviderHealth, len(providers))
	listing := make([]entity.LoginMethodView, 0, len(providers))

	// Counted rather than decided in the loop: the rule is about the whole set
	// -- Secure unless EVERY dance-capable provider comes back over http -- so
	// it can only be settled once all of them have been seen.
	danceable, insecure := 0, 0

	for _, p := range providers {
		if p.Method != nil {
			methods[p.ID] = p.Method
		}
		if p.Gateway != nil {
			gateways[p.ID] = p.Gateway

			danceable++
			if redirectsOverPlainHTTP(p.RedirectURI) {
				insecure++
			}
		}
		health[p.ID] = p.Health

		// A provider that did not resolve is still listed: this reports what is
		// configured, not what is reachable, and hiding it would make a brief
		// IdP outage look like a deleted provider. A disabled one is not listed
		// -- an operator turned it off deliberately.
		if p.Health != entity.LoginProviderHealthDisabled {
			listing = append(listing, entity.LoginMethodView{ID: p.ID, DisplayName: p.DisplayName})
		}
	}

	// Sorted, and this is a contract rather than tidiness: two callers of
	// /auth/providers must get identical bytes, and map iteration is randomized,
	// so an unsorted listing would reshuffle the buttons between requests and
	// between replicas.
	sort.Slice(listing, func(i, j int) bool { return listing[i].ID < listing[j].ID })

	// No dance-capable provider counts as secure, the same fail-safe direction:
	// a cookie the browser withholds beats one it leaks, and the reverse mistake
	// costs only a local sign-in where http is configured at all.
	danceCookieSecure := danceable == 0 || insecure < danceable

	return &snapshot{
		danceCookieSecure: danceCookieSecure,
		methods:           methods,
		gateways:          gateways,
		listing:           listing,
		health:            health,
	}
}

// redirectsOverPlainHTTP reports whether a provider comes back over http, which
// is what decides the dance cookies' Secure attribute.
//
// Anything it cannot read counts as https, matching the fail-safe direction of
// the rule it feeds: a cookie the browser withholds costs a local sign-in, one
// it leaks over http costs the session.
func redirectsOverPlainHTTP(redirectURI string) bool {
	parsed, err := url.Parse(redirectURI)

	return err == nil && parsed.Scheme == "http"
}

// healthOf reports a provider's state; unknown providers read as unresolved,
// which is what "listed, and /start refuses" already means -- a row that exists
// in the database but is missing from the snapshot is in exactly that state.
func (s *snapshot) healthOf(id entity.AuthMethod) entity.LoginProviderHealth {
	if h, ok := s.health[id]; ok {
		return h
	}

	return entity.LoginProviderHealthUnresolved
}

// providerInput describes one configured provider to installProviders.
type providerInput struct {
	ID          entity.AuthMethod
	DisplayName string
	// Method verifies ID tokens from this provider. Never nil for a provider
	// that should be usable at all.
	Method AuthMethod
	// Gateway runs the confidential half. Nil means the provider is listed and
	// verifiable but cannot start a backend dance -- a BFF-only instance, or one
	// whose credentials are not configured.
	Gateway Gateway
	Health  entity.LoginProviderHealth
	// RedirectURI is where this provider sends the browser back. Carried as the
	// URL rather than as a precomputed "is it http" flag: whether the dance
	// cookies keep Secure is a property of the whole provider SET, so it can
	// only be settled once every provider has been seen -- and that happens in
	// newSnapshot, at rebuild time, not on the request path.
	RedirectURI string
}

// installProviders replaces the live configuration with one built from the
// given providers, keeping the built-in methods that are not configured through
// the registry.
//
// Built-ins are carried over from the current snapshot rather than rebuilt, and
// they are identified by exclusion: anything in the previous snapshot that the
// incoming set does not name is not a provider. That is deliberate — a list of
// built-in ids would be a second thing to update whenever one is added, and
// forgetting it would silently drop the dev stub, which Get substitutes for any
// method on the stands that ship use_stub.
func (p *Methods) installProviders(providers []providerInput) {
	// The built-ins are the ones this was constructed with, not whatever the
	// outgoing snapshot happened to hold. They never change: nothing can add a
	// method that is not configured through the registry, so a rebuild replaces
	// the provider half entirely and leaves the other half exactly as it began.
	p.replace(newSnapshot(p.builtins, providers))
}

// ProviderHealth reports one provider's state as a plain string.
//
// A string rather than entity.LoginProviderHealth because of where the value
// goes: straight into the admin API's wire model, whose Health field is a
// string with omitempty, so a non-login row can be absent from the response.
// Handing out the typed value would only be converted back one call later.
//
// A name with no entry reads as unresolved rather than empty: a row that exists
// in the registry but never made it into a snapshot is in exactly the state
// that value already describes -- listed, and refusing sign-in.
func (p *Methods) ProviderHealth(name string) string {
	return string(p.snapshot().healthOf(entity.AuthMethod(name)))
}

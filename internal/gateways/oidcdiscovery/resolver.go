// Package oidcdiscovery resolves an OIDC provider from its issuer and keeps the
// result.
//
// The protocol work -- fetching {issuer}/.well-known/openid-configuration,
// parsing it, and checking that the document speaks for the issuer it was
// fetched from -- belongs to github.com/coreos/go-oidc. What this package adds
// is the operational behavior that library deliberately leaves to its caller,
// because it cannot know our answers:
//
//   - an unreachable IdP must not abort startup, so resolution is retried on
//     use rather than required at construction;
//   - a burst of sign-ins against a cold cache must produce one outbound
//     request, not one per sign-in;
//   - a sustained outage must not turn every arriving request into a full
//     outbound timeout;
//   - an endpoint the document names must be https, since the client secret
//     travels to one of them.
package oidcdiscovery

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ruko1202/xhttp/client"

	"github.com/ruko1202/maintmode/internal/utils/xcache"
	"github.com/ruko1202/maintmode/internal/utils/xsanitize"
)

// requestTimeout bounds one discovery round trip.
//
// Sized like the token exchange rather than like a call to our own services:
// this crosses the public internet, and a tight ceiling would manufacture
// failures out of ordinary latency. The other bound is the server's own request
// context — without a deadline here an unresponsive IdP pins the handler.
const requestTimeout = 5 * time.Second

// defaultCoolOff is how long a fresh failure suppresses another attempt.
//
// Discovery sits on an unauthenticated sign-in route, so without it a
// sustained IdP outage turns every arriving request into its own outbound
// timeout. It is short enough that an IdP coming back is picked up on the next
// sign-in rather than needing a restart.
const defaultCoolOff = 10 * time.Second

// cacheForever is the TTL for a resolved provider: longer than any process
// lives, which is how "cached for the life of the process" is expressed to a
// cache that only knows TTLs.
const cacheForever = 100 * 365 * 24 * time.Hour

// Provider is a resolved OIDC provider: the library's own handle, which carries
// the endpoints and mints verifiers, plus the issuer as the document declared
// it.
//
// Issuer is exposed because it is not always the configured string — a config
// may carry a trailing slash the document does not — and ID tokens must be
// checked against what the provider actually mints.
type Provider struct {
	OIDC   *oidc.Provider
	Issuer string
}

// Endpoint returns the OAuth2 endpoints the discovery document declared.
func (p Provider) Endpoint() (authURL, tokenURL string) {
	endpoint := p.OIDC.Endpoint()

	return endpoint.AuthURL, endpoint.TokenURL
}

// Resolver resolves providers and remembers the ones that worked.
//
// Successes are cached for the life of the process: endpoints effectively never
// move, and key rotation is the library's JWKS refresh, not this cache's
// business. Failures are not cached as results — an IdP that was down at
// startup must be usable once it returns — but a fresh one is remembered just
// long enough to keep an outage from being amplified.
type Resolver struct {
	httpc *http.Client

	// resolved carries the cache AND the single-flight: GetOrLoad coalesces
	// concurrent misses, which is what keeps a burst of sign-ins against a cold
	// cache from becoming a burst of identical discovery requests.
	resolved *xcache.Cache[string, Provider]
	// failedAt is a set, not a map: the TTL is the cool-off, so an entry's
	// presence IS "this issuer failed recently" and there is no timestamp to
	// compare. Storing one would suggest a comparison that never happens.
	failedAt *xcache.Cache[string, struct{}]
}

// New builds a resolver with the default cool-off.
func New() *Resolver {
	return NewWithCoolOff(defaultCoolOff)
}

// NewWithCoolOff builds a resolver whose failure cool-off is coolOff. Zero
// disables the cool-off, which is what a test driving a recovering IdP needs;
// production uses New.
func NewWithCoolOff(coolOff time.Duration) *Resolver {
	// A cool-off entry expires by itself, so the TTL IS the cool-off and there
	// is nothing to sweep.
	if coolOff <= 0 {
		coolOff = time.Nanosecond
	}

	return &Resolver{
		httpc: client.NewClient(
			client.WithTimeout(requestTimeout),
			client.WithSanitizer(xsanitize.New()),
		),
		// cacheForever: a successful discovery is good for the life of the
		// process, so the TTL is one the process will never outlive.
		resolved: xcache.New[string, Provider](cacheForever),
		failedAt: xcache.New[string, struct{}](coolOff),
	}
}

// Resolve returns the provider at issuerURL, discovering it once and serving
// every later caller from cache.
//
// Concurrent first callers collapse into a single outbound request: /start is
// unauthenticated, so a burst of sign-ins against a cold cache would otherwise
// be a burst of identical fetches.
func (r *Resolver) Resolve(ctx context.Context, issuerURL string) (Provider, error) {
	issuer := strings.TrimSuffix(issuerURL, "/")

	return r.resolved.GetOrLoad(issuer, func() (Provider, error) {
		if _, coolingOff := r.failedAt.Get(issuer); coolingOff {
			return Provider{}, fmt.Errorf("discovery for %s failed recently", issuer)
		}

		provider, err := r.discover(ctx, issuer)
		if err != nil {
			r.failedAt.Set(issuer, struct{}{}, r.failedAt.Generation(issuer))

			return Provider{}, err
		}

		r.failedAt.Invalidate(issuer)

		return provider, nil
	})
}

// discover runs the library's discovery and then applies the one check it does
// not make.
func (r *Resolver) discover(ctx context.Context, issuer string) (Provider, error) {
	// The fetch deliberately does NOT inherit the caller's cancellation.
	//
	// Whoever wins the flight fetches on behalf of everyone waiting on it, and
	// those waiters have their own live contexts. If the winner is an HTTP
	// handler whose browser went away, its canceled context would fail the
	// shared fetch and singleflight would hand that failure to every waiter --
	// and the failure would then start a cool-off, so one user pressing Escape
	// would refuse sign-ins for the next ten seconds against a healthy IdP.
	//
	// The deadline is this package's own, which is what the budget was always
	// meant to be. ClientContext is how the library is told to use our HTTP
	// client rather than http.DefaultClient, which carries neither our timeout
	// nor our log redaction.
	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), requestTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(oidc.ClientContext(fetchCtx, r.httpc), issuer)
	if err != nil {
		return Provider{}, fmt.Errorf("discover %s: %w", issuer, err)
	}

	// The library checks that the document's issuer matches the one we asked
	// about (RFC 8414) and refuses otherwise, so by here the two agree.
	if err := validateEndpoints(provider, issuer); err != nil {
		return Provider{}, err
	}

	// The library refused the document unless its issuer equaled the string we
	// asked with, so the normalized issuer IS what the provider mints.
	return Provider{OIDC: provider, Issuer: issuer}, nil
}

// discoveredEndpoints is what the document has to supply, named so validation
// errors say which one was wrong.
type discoveredEndpoints struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// validateEndpoints rejects a document that would send a credential in the
// clear.
//
// The library validates the issuer but not the transport of the endpoints it
// hands back. The client secret goes to the token endpoint and the signing keys
// come from the JWKS one, so all three must be reachable over TLS.
func validateEndpoints(provider *oidc.Provider, issuer string) error {
	var endpoints discoveredEndpoints
	if err := provider.Claims(&endpoints); err != nil {
		return fmt.Errorf("read discovery claims for %s: %w", issuer, err)
	}

	secureURL := secureEndpointRule()

	if err := validation.ValidateStruct(&endpoints,
		validation.Field(&endpoints.AuthorizationEndpoint, validation.Required, secureURL),
		validation.Field(&endpoints.TokenEndpoint, validation.Required, secureURL),
		validation.Field(&endpoints.JWKSURI, validation.Required, secureURL),
	); err != nil {
		return fmt.Errorf("discovery document for %s: %w", issuer, err)
	}

	return nil
}

// secureEndpointRule is the rule an endpoint URL has to satisfy: absolute, and
// https.
//
// https is not a formality. The client secret goes to the token endpoint, and
// the signing keys that decide whether a token is genuine come from the JWKS
// one -- over plain http both are rewritable by anyone on the path. There is
// deliberately no escape hatch, not even for loopback or for dev: an earlier
// version had one, it existed only because the test fixtures served plain http,
// and those fixtures now mock the resolver instead.
func secureEndpointRule() validation.Rule {
	return validation.By(func(value any) error {
		raw, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected a url string, got %T", value)
		}

		parsed, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("is not a url: %w", err)
		}

		if parsed.Host == "" {
			return fmt.Errorf("must be an absolute url, got %q", raw)
		}

		if parsed.Scheme != "https" {
			return fmt.Errorf("must be an https url, got %q", raw)
		}

		return nil
	})
}

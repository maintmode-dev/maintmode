// Package authmethod holds the live login configuration and keeps it in step
// with the integration registry.
//
// Both halves live here because they are one thing: the snapshot is what
// readers serve, and the reloader is what rebuilds it after an operator's save
// takes effect without a restart. Split across two packages, every type the
// reloader handed the snapshot had to be exported for the other side to name.
package authmethod

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	oidcgw "github.com/ruko1202/maintmode/internal/gateways/oidc"
	"github.com/ruko1202/maintmode/internal/gateways/oidcdiscovery"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/services/authmethod/oidc"
)

// tickInterval bounds how long a replica that did not observe a write keeps
// serving the previous configuration.
//
// The write lands on one replica, which reloads at once; the others learn on
// their own tick. Thirty seconds matches the staleness ceiling delivery already
// accepts for the same reason, and keeps a "turn this provider off" from taking
// visibly long fleet-wide.
const tickInterval = 30 * time.Second

// Registry is the half of the integration service this needs. Declared
// consumer-side, together with the type it carries, so this package does not
// import services/integration at all -- the claim is checkable, and it was not
// true while the interface lived here and its return type did not.
//
// The direction is deliberate. The registry reads rows, decrypts secrets and
// runs transactions; the reloader needs four fields out of that. Naming what it
// needs keeps the dependency pointing at a contract rather than at a service,
// and it is the same arrangement as providerInput one step further
// down this pipeline.
type Registry interface {
	ListLoginProviders(ctx context.Context, kind string) ([]entity.ConfiguredProvider, error)
}

// installer receives a rebuilt provider set. Implemented by Methods.
type installer interface {
	installProviders(providers []providerInput)
}

// discoveryResolver is the discovery half a built provider needs, declared
// consumer-side -- the same shape oidc.Service and the token-exchange client
// each declare for themselves.
//
// The reloader never calls it: it hands the resolver to the two halves it
// builds. Naming the contract rather than the concrete *oidcdiscovery.Resolver
// keeps the shared cache out of this package's signatures.
type discoveryResolver interface {
	Resolve(ctx context.Context, issuerURL string) (oidcdiscovery.Provider, error)
}

// Reloader rebuilds the login snapshot from the registry.
type Reloader struct {
	registry  Registry
	installer installer
	discovery discoveryResolver
	// wake carries a pending rebuild. Buffered to one, so N changes arriving
	// together collapse into a single reload -- the rebuild reads everything
	// anyway.
	wake chan struct{}
}

// NewReloader builds a reloader over the registry and the live method set.
//
// Named for what it makes rather than plain New, because this package holds
// two constructors now: NewAuthMethods builds the thing that serves the
// configuration, this one builds the thing that refreshes it.
func NewReloader(
	registry Registry,
	installer installer,
	discovery discoveryResolver,
) *Reloader {
	return &Reloader{
		registry:  registry,
		installer: installer,
		discovery: discovery,
		wake:      make(chan struct{}, 1),
	}
}

// OnIntegrationChanged is the hook the registry calls after a committed write.
//
// It only schedules work, for three reasons that outlive the one this comment
// used to give. (Rebuilding no longer reaches the network at all, so "discovery
// would hold the response open" stopped being true when the warm-up went.)
//
// The registry calls its listeners SYNCHRONOUSLY, inside the admin's request.
// A rebuild reads every login row and unwraps a data key per row to decrypt its
// secret, so running it here would put that work in a POST that changed one
// provider, and make it scale with how many others exist.
//
// The channel also coalesces: a burst of writes collapses into one rebuild,
// because the rebuild reads everything anyway. Calling reload directly would do
// the whole job once per changed row.
//
// And a rebuild that fails signals again, so the loop retries at once instead
// of waiting out the tick. Inline, that retry would be recursion inside the
// handler.
func (r *Reloader) OnIntegrationChanged(kind, _ string) {
	// The CATEGORY, not a provider name. Before login rows could carry more
	// than one system name, "oidc" and "a login provider" were the same test;
	// now comparing against a name would mean a change to any other provider
	// never reaches the snapshot, and "no restart" would silently stop holding
	// for it.
	if kind != integrationkinds.CategoryLogin {
		return
	}

	r.signal()
}

// Run loads the configuration, then keeps it current until ctx is done.
//
// The FIRST rebuild happens synchronously, before this returns: the caller
// wires the auth handlers immediately afterwards, so a process that started
// serving before the snapshot was built would answer its first requests with
// no providers -- a sign-in page whose SSO buttons are missing for as long as
// the first rebuild takes. Everything after that is a background loop.
//
// The loop is a plain goroutine, deliberately NOT wrapped in recover() and
// deliberately not started from a closer-registered function, which recovers
// panics in its handlers. A panic there must take the process down, where
// crash-loop alerting sees it: a recovered panic would leave a live process
// serving a frozen snapshot with every availability signal green, which is the
// failure mode nobody notices.
func (r *Reloader) Run(ctx context.Context) {
	r.reload(ctx)

	go r.loop(ctx)
}

// loop rebuilds on every change and on a tick until ctx is done.
func (r *Reloader) loop(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
			r.reload(ctx)
		case <-ticker.C:
			// The tick is not only for other replicas: a provider whose IdP was
			// unreachable at the last rebuild gets another chance here, without
			// anyone having to touch its configuration.
			r.reload(ctx)
		}
	}
}

func (r *Reloader) reload(ctx context.Context) {
	stored, err := r.registry.ListLoginProviders(ctx, integrationkinds.CategoryLogin)
	if err != nil {
		// The previous snapshot stays live. Falling back to "no providers"
		// because the database blinked would turn a registry outage into a
		// login outage.
		//
		// The wake is re-armed rather than left to the tick: the change that
		// prompted this is still unapplied, so the loop comes straight back to
		// it instead of treating the failure as the end of the matter.
		xlog.Error(ctx, "login provider reload failed", xfield.Error(err))
		r.signal()

		return
	}

	r.installer.installProviders(r.build(ctx, stored))
}

// signal nudges the loop, coalescing repeats: the rebuild reads everything
// anyway, so N changes collapse into one reload.
//
// This is also what survives a FAILED rebuild. A reload that cannot read the
// registry calls signal again, so the change that prompted it is retried at
// once rather than waiting out the tick with nothing indicating why.
func (r *Reloader) signal() {
	select {
	case r.wake <- struct{}{}:
	default: // a wake-up is already pending; one rebuild covers both
	}
}

// build turns the stored rows into the snapshot input.
//
// The registry is the only source. Providers used to also arrive from the
// config file and win a name collision with a stored row -- the break-glass
// path for an operator locked out of the admin UI. That half is gone with the
// config section, and the bootstrap admin is the remaining way in.
func (r *Reloader) build(ctx context.Context, stored []entity.ConfiguredProvider) []providerInput {
	inputs := make([]providerInput, 0, len(stored))
	for _, row := range stored {
		inputs = append(inputs, r.buildOne(ctx, entity.AuthMethod(row.Name), row))
	}

	return inputs
}

func (r *Reloader) buildOne(
	ctx context.Context, id entity.AuthMethod, row entity.ConfiguredProvider,
) providerInput {
	input := providerInput{ID: id, DisplayName: row.Name}

	stored, isOIDC := row.Settings.(integrationkinds.OIDCSettings)

	switch {
	case row.Unreadable:
		// Not logged again here: ListLoginProviders already reported it where it
		// arose, with the provider named and the error object left behind -- a
		// decrypt failure's text can carry secret material.
		input.Health = entity.LoginProviderHealthUnreadable

	case !row.Enabled:
		// Neither method nor gateway: a disabled provider must stop working,
		// not merely stop being listed.
		input.Health = entity.LoginProviderHealthDisabled

	case !isOIDC:
		// Reading the row is all this does, so reaching here means the stored
		// settings are not the shape a login provider has -- a delivery row
		// sitting in the login category, say. It is a per-provider failure:
		// that one disables ITSELF and stays listed with its state, and every
		// other provider in this rebuild is unaffected.
		xlog.Error(ctx, "login provider did not build",
			xfield.String("provider", row.Name),
			xfield.String("settings_type", fmt.Sprintf("%T", row.Settings)))
		input.Health = entity.LoginProviderHealthUnreadable

	default:
		// A fresh *oidc.Service per rebuild, never a reused one: its verifier is
		// cached for the life of the object under the issuer, and the client id
		// it was built with is the audience it checks. Reusing one across a
		// client_id change would keep accepting tokens minted for the previous
		// registration.
		//
		// No discovery here. This reads the stored row and nothing else, so a
		// rebuild cannot block on an IdP or fail because one is down -- the
		// snapshot is the configuration, not a probe of the world it names. The
		// verifier resolves lazily on first use instead: the resolver caches per
		// issuer, so the first sign-in pays for the fetch and every later one is
		// served warm.
		provider := config.OIDCProvider{
			DisplayName:  stored.DisplayName,
			IssuerURL:    stored.IssuerURL,
			ClientID:     stored.ClientID,
			ClientSecret: stored.ClientSecret,
			RedirectURI:  stored.RedirectURI,
			Scopes:       stored.Scopes,
			JWTVerify: config.JWTVerifierConfig{
				// The only provider-facing field of that block the OIDC path
				// reads, and an authorization control: an empty list means no
				// domain restriction, so losing it here fails OPEN.
				AllowedHostedDomains: stored.JWTVerify.AllowedHostedDomains,
			},
		}

		input.Method = oidc.NewProvider(row.Name, provider, r.discovery)
		input.Gateway = oidcgw.NewClient(provider, r.discovery)
		// The operator's label, falling back to the instance name so a button is
		// never blank.
		input.DisplayName = cmp.Or(stored.DisplayName, row.Name)
		input.RedirectURI = stored.RedirectURI
		input.Health = entity.LoginProviderHealthOK
	}

	return input
}

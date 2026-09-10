// Package oidc verifies ID tokens issued by any OpenID Connect provider.
//
// One implementation serves them all — Google, Keycloak, Okta, Azure AD,
// Authentik — because the discovery document at the issuer names the endpoints
// and the JWKS location, so nothing about a provider needs to be known at
// compile time. Instances differ only by issuer and credentials, which is what
// makes corporate SSO a line of configuration rather than a plugin.
//
// The protocol work is github.com/coreos/go-oidc's: signature, audience,
// issuer and expiry are its verifier's business. What is left here is the part
// specific to this backend — which claims we trust, and the refusal that
// belongs to us rather than to the token's shape.
//
// This half never talks to the token endpoint: no client secret, no outbound
// call beyond discovery and JWKS. gateways/oidc is the confidential-client half
// and holds the secret. The split is deliberate — verification is offline,
// exchange is a network call with a credential — and this half is reachable
// from BOTH sign-in paths, so the audience and issuer checks have one home.
package oidc

import (
	"context"
	"fmt"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/oidcdiscovery"
	"github.com/ruko1202/maintmode/internal/utils/xcache"
)

// verifierCacheTTL is longer than any process lives: a verifier is good until
// the process ends, and the cache only speaks TTLs.
const verifierCacheTTL = 100 * 365 * 24 * time.Hour

// resolver is the discovery half this provider needs, declared here so the
// consumer owns the contract.
type resolver interface {
	Resolve(ctx context.Context, issuerURL string) (oidcdiscovery.Provider, error)
}

// Service verifies ID tokens for one configured instance.
type Service struct {
	name     entity.AuthMethod
	clientID string
	issuer   string
	cfg      config.JWTVerifierConfig

	discovery resolver

	// The verifier cannot be built until discovery has named the issuer and the
	// JWKS location, and discovery may be unreachable when the process starts.
	// Rather than abort the boot or defer every instance, the provider is
	// registered either way and completes itself on first use.
	//
	// A one-entry cache rather than a sync.Once: Once would burn its single run
	// on a failed attempt, and an instance whose IdP was down at startup would
	// then stay dead until the process restarted -- the opposite of what
	// registering it unresolved is for. GetOrLoad keeps the success, drops the
	// failure, and coalesces concurrent first verifies into one build.
	verifier *xcache.Cache[string, *gooidc.IDTokenVerifier]
}

// NewProvider creates a provider for the instance named name.
//
// It does not reach the network. Discovery is attempted by Warm at startup and,
// failing that, on first use — so an IdP that is down when the process starts
// costs that instance's sign-ins, not the boot.
func NewProvider(name string, cfg config.OIDCProvider, discovery resolver) *Service {
	return &Service{
		name:      entity.AuthMethod(name),
		clientID:  cfg.ClientID,
		issuer:    cfg.IssuerURL,
		cfg:       cfg.JWTVerify,
		discovery: discovery,
		// A verifier is good for the life of the process: key rotation is the
		// library's JWKS refresh, not this cache's business.
		verifier: xcache.New[string, *gooidc.IDTokenVerifier](verifierCacheTTL),
	}
}

func (s *Service) MethodID() entity.AuthMethod {
	return s.name
}

// Warm resolves discovery ahead of first use.
//
// Startup calls this for every instance and ignores the error beyond logging
// it: a provider whose IdP is unreachable must not stop the others, or the
// process, from coming up.
func (s *Service) Warm(ctx context.Context) error {
	_, err := s.resolveVerifier(ctx)

	return err
}

// verifier returns this instance's token verifier, building it on first use.
//
// Discovery names the issuer and the JWKS location, so nothing can be built
// before it answers -- and it may not answer at all when the process starts.
func (s *Service) resolveVerifier(ctx context.Context) (*gooidc.IDTokenVerifier, error) {
	return s.verifier.GetOrLoad(s.issuer, func() (*gooidc.IDTokenVerifier, error) {
		provider, err := s.discovery.Resolve(ctx, s.issuer)
		if err != nil {
			return nil, fmt.Errorf("resolve discovery for %s: %w", s.name, err)
		}

		xlog.Info(ctx, "oidc provider resolved", xfield.String("provider", string(s.name)))

		// SupportedSigningAlgs is set rather than left to the library's default
		// so the accepted set is the one this backend decided on, not one that
		// widens when a dependency updates.
		//
		// The key set uses context.Background(): it outlives whichever request
		// happened to trigger this, and its refreshes must not be canceled when
		// that request ends.
		return provider.OIDC.VerifierContext(context.Background(), &gooidc.Config{
			ClientID:             s.clientID,
			SupportedSigningAlgs: []string{gooidc.RS256, gooidc.ES256},
		}), nil
	})
}

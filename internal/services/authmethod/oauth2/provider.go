// Package oauth2 asserts the identity behind an OAuth 2.0 access token.
//
// It is the identity half of a plain OAuth 2.0 sign-in and the sibling of
// services/authmethod/oidc, not a variant of it. The two answer the same
// question -- "who does this credential belong to?" -- from opposite ends:
// the OIDC provider verifies a signed id_token offline against a JWKS and
// needs no secret and no network, while this one holds nothing and asks the
// vendor, because a plain access token is opaque and there is no signature
// to check.
//
// That asymmetry is why the trust comes from elsewhere. The token reaching
// Authenticate was obtained by the dance using THIS app's client_secret, so
// what makes the answer trustworthy is the exchange that produced it, not
// anything verifiable about the string itself. A token minted for any other
// OAuth app authenticates against the same endpoint identically -- which is
// exactly why a raw access token is never accepted from a client.
//
// It is vendor-agnostic: which endpoints answer and which address may be
// trusted belongs to the gateway's vendor, and this package never learns
// either.
package oauth2

import (
	"context"
	"fmt"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

// identityGateway is the half of the OAuth 2.0 gateway this provider needs,
// declared here so the consumer owns the contract -- the same shape
// services/authmethod/oidc uses for its discovery resolver.
type identityGateway interface {
	FetchIdentity(ctx context.Context, accessToken string) (*entity.OAuth2Identity, error)
}

// Service is one configured OAuth 2.0 instance's auth method.
type Service struct {
	// name is the configured INSTANCE name, and it is the method id: it keys
	// this instance in the registry, is what a request names in the provider
	// path segment, and is what reaches user_identities.provider.
	name    entity.AuthMethod
	gateway identityGateway
}

// NewProvider builds the auth method for one instance.
//
// No Warm, unlike the OIDC provider: there is no discovery document to resolve,
// so an instance is complete the moment it is constructed and nothing has to
// finish itself on first use.
func NewProvider(name string, gateway identityGateway) *Service {
	return &Service{name: entity.AuthMethod(name), gateway: gateway}
}

// MethodID returns the method this instance authenticates as.
//
// The INSTANCE NAME, exactly as the OIDC provider does it, and not a per-vendor
// constant. The registry is keyed by this value and a callback names it in the
// URL path, so a constant would mean two instances of the same vendor -- a
// github.com app and a GitHub Enterprise Server one -- collapsing onto one key,
// where whichever was built last silently replaces the other.
//
// It is also what reaches user_identities.provider, which is why an instance
// named "github" writes exactly the string it always wrote.
func (s *Service) MethodID() entity.AuthMethod {
	return s.name
}

// Authenticate trades an access token for the claims that identify a user.
//
// The credential here is an access token, where the OIDC implementation receives
// an id_token. The AuthMethod interface types it as a plain string precisely
// because its meaning belongs to the implementation, and a provider is only ever
// handed its own gateway's output.
func (s *Service) Authenticate(ctx context.Context, credential string) (*entity.OAuthIDTokenClaims, error) {
	// An empty credential can only come from a bug on our side: Exchange refuses
	// to return one. Refused here rather than spent on an outbound request that
	// is guaranteed to fail.
	if credential == "" {
		return nil, fmt.Errorf("%w: empty oauth2 credential", apperr.ErrInvalidCredentials)
	}

	identity, err := s.gateway.FetchIdentity(ctx, credential)
	if err != nil {
		// Wrapped with %w and nothing else interpreted: the dance path tells an
		// unusable email apart from a provider outage by matching the gateway's
		// sentinels, and an audit reason depends on that distinction surviving.
		return nil, fmt.Errorf("fetch oauth2 identity: %w", err)
	}

	return &entity.OAuthIDTokenClaims{
		Subject: identity.Subject,
		Email:   identity.Email,
		Name:    identity.Name,
		// EXPLICITLY true, and not cosmetic. The vendor already refused any
		// address that is not both primary and verified, so by the time an
		// identity exists the check has been made -- this reports it rather than
		// assuming it.
		//
		// Leaving it at the zero value would be read downstream as "the issuer
		// reports this address unverified": services/invitation/accept.go
		// refuses that with ErrEmailMismatch, so every invited sign-in would fail
		// with "that address does not match" on an address that matches exactly.
		// The same reasoning is why bootstrapauth and the dev stub set it true --
		// a provider with no issuer to vouch must vouch for itself.
		EmailVerified: true,
	}, nil
}

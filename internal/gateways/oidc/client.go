// Package oidc is the gateway to an OIDC provider's token endpoint: one
// exchange per sign-in trades an authorization code for an id_token.
//
// It is the confidential-client half of sign-in, and the only one that needs
// the client secret. The verification half lives in services/authmethod/oidc
// and talks to nobody but the provider's JWKS.
//
// Endpoints are not configured: they come from the provider's discovery
// document, which is why the oauth2.Config is assembled per call rather than
// once at construction.
//
// The exchange itself is golang.org/x/oauth2 rather than a hand-rolled POST:
// the grant type, the form encoding, the RFC 6749 error shape and the PKCE
// parameter are all its business, and reimplementing them buys nothing. What
// this package adds is the transport — oauth2 takes its *http.Client from the
// context, so the project's xhttp client goes in there and the exchange inherits
// the same timeout and log-redaction policy as every other outbound call.
package oidc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/ruko1202/xhttp/client"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/gateways/oidcdiscovery"
	"github.com/ruko1202/maintmode/internal/utils/xsanitize"
)

// exchangeTimeout bounds one token-endpoint round trip.
//
// Deliberately far more generous than the license gateway's one-second
// fallback: that client calls our own Console over a short hop, this crosses
// the public internet. A tight ceiling would manufacture failures out of
// ordinary latency, and an aborted exchange costs the user the whole dance.
//
// The other bound is the server's 60s context timeout — without a deadline
// here, an unresponsive provider pins the callback handler for that minute.
const exchangeTimeout = 10 * time.Second

// defaultScopes is what an OIDC sign-in needs: an id_token, and the two claims
// this backend reads off it. Overridable per instance for providers that gate a
// claim behind an extra scope.
var defaultScopes = []string{"openid", "email", "profile"}

// resolver is the discovery half this gateway needs, declared here so the
// consumer owns the contract.
type resolver interface {
	Resolve(ctx context.Context, issuerURL string) (oidcdiscovery.Provider, error)
}

// Client talks to one instance's token endpoint.
type Client struct {
	provider  config.OIDCProvider
	scopes    []string
	discovery resolver
	httpc     *http.Client
}

// NewClient builds the token-exchange client for one instance. The request
// timeout is the gateway's own business, not the caller's — see exchangeTimeout.
func NewClient(provider config.OIDCProvider, discovery resolver) *Client {
	scopes := provider.Scopes
	if len(scopes) == 0 {
		scopes = defaultScopes
	}

	return &Client{
		provider:  provider,
		scopes:    scopes,
		discovery: discovery,
		httpc: client.NewClient(
			client.WithTimeout(exchangeTimeout),
			client.WithSanitizer(xsanitize.New()),
		),
	}
}

// oauthConfig assembles the library config for this instance, resolving the
// endpoints from discovery.
//
// Per call rather than per construction because discovery may not have answered
// yet when the process starts, and an instance that resolves later must work
// without a restart.
func (c *Client) oauthConfig(ctx context.Context) (oauth2.Config, error) {
	provider, err := c.discovery.Resolve(ctx, c.provider.IssuerURL)
	if err != nil {
		return oauth2.Config{}, fmt.Errorf("%w: %w", apperr.ErrAuthUnavailable, err)
	}

	return oauth2.Config{
		ClientID:     c.provider.ClientID,
		ClientSecret: c.provider.ClientSecret,
		RedirectURL:  c.provider.RedirectURI,
		Endpoint:     provider.OIDC.Endpoint(),
		Scopes:       c.scopes,
	}, nil
}

// AuthCodeURL builds the provider redirect /start sends the browser to.
//
// It lives here rather than in the handler because oauth2.Config already holds
// the client id, the redirect URI and the endpoint: assembling the same URL by
// hand in the API layer meant a second copy of all three, plus the response_type
// and challenge-method literals the library sets itself.
//
// It returns an error because the endpoint is no longer known at construction:
// an unresolved instance has no authorization endpoint, and returning a URL
// without a host would send the browser nowhere as a 302 that reads as success
// in every access log.
//
// Only the CHALLENGE goes out; the verifier stays in Valkey, which is what stops
// an intercepted authorization code from being redeemable.
func (c *Client) AuthCodeURL(ctx context.Context, state, verifier string) (string, error) {
	cfg, err := c.oauthConfig(ctx)
	if err != nil {
		return "", err
	}

	return cfg.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
	), nil
}

// Package oauth2 is the gateway to one configured OAuth 2.0 app: it trades an
// authorization code for an access token, and trades that token for the identity
// behind it.
//
// It is the confidential-client half of an OAuth 2.0 sign-in, and the only half
// that holds the client secret. The identity-assertion half lives in
// services/authmethod/oauth2 and talks to nobody.
//
// It does MORE than the OIDC gateway on purpose, and the asymmetry is the
// protocol's, not a design choice. An OIDC exchange returns an id_token that
// carries the claims and can be verified offline against a JWKS; plain OAuth 2.0
// returns an opaque access token whose meaning is only knowable by asking the
// vendor's API. So the identity reads that OIDC gets for free are network calls
// here, and the identity they resolve is trusted because THIS app's
// client_secret obtained the token -- there is nothing to verify a signature
// against.
//
// The split inside this package is the one that matters. Everything here is the
// authorization-code grant, which is identical at every vendor; everything that
// differs -- which endpoints answer "who is this", what they return, and which
// address may be trusted -- is behind the Vendor interface. Adding a vendor
// means writing a Vendor, not touching this file.
//
// Endpoints are configured rather than discovered: plain OAuth 2.0 publishes no
// discovery document. They default to the vendor's hosted deployment and exist
// as overrides for self-hosted installations, which are deliberately untested.
package oauth2

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/ruko1202/xhttp/client"
	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xsanitize"
)

// exchangeTimeout bounds one token-endpoint round trip.
//
// The OIDC gateway's value, for the OIDC gateway's reason: this crosses the
// public internet, and an aborted exchange costs the user the whole dance.
const exchangeTimeout = 10 * time.Second

// totalBudget bounds a vendor's identity reads together.
//
// Scope stated precisely, because an earlier version of this comment claimed it
// covered the token exchange as well and it never did: Exchange is a separate
// method, invoked earlier by the dance, under its own exchangeTimeout. The whole
// sign-in is therefore bounded by exchangeTimeout + totalBudget, not by this
// number alone.
//
// BELOW the sum of the two per-call bounds a two-read vendor spends (5+5), so it
// is a constraint that can actually fire rather than a number describing the
// others. At 15s it could not: two reads capped at 5s each can never reach it,
// which made it decorative and its test vacuous -- the test passed with the
// budget deleted, because apiTimeout ended the call first.
//
// Applied here rather than by the caller: the callback has no reason to know how
// many round trips a vendor makes, and a vendor must not be able to widen it.
const totalBudget = 8 * time.Second

// Client talks to one configured OAuth 2.0 app.
type Client struct {
	provider entity.OAuth2Credentials
	vendor   Vendor
	// tokenHTTPC serves the token exchange, which is the only call this layer
	// makes: the identity reads belong to the vendor, along with the client that
	// makes them.
	tokenHTTPC *http.Client
}

// NewClient builds the gateway for one instance.
//
// No network at construction and nothing to warm: unlike OIDC there is no
// discovery document to resolve, so an instance is usable the moment it is
// built.
//
// The vendor arrives resolved rather than being looked up from the config
// string here, so an unsupported vendor is refused at startup by whoever owns
// the registry instead of producing a nil resolver that fails at first sign-in.
func NewClient(provider entity.OAuth2Credentials, vendor Vendor) *Client {
	return &Client{
		provider: provider,
		vendor:   vendor,
		tokenHTTPC: client.NewClient(
			client.WithTimeout(exchangeTimeout),
			client.WithSanitizer(xsanitize.New()),
			// Accept: application/json is not optional, and it is a HEADER.
			// GitHub's token endpoint answers in form-encoded by default; oauth2
			// does read that dialect, but relying on it would leave this gateway
			// depending on a fallback rather than on what it asked for. An
			// "accept" FORM parameter looks like it does the same job and does
			// not -- GitHub reads the header. It is sent for every vendor
			// because a JSON token response is what RFC 6749 specifies, so
			// asking for it is never wrong.
			//
			// The hook is the library's own seam for this: oauth2 builds the
			// token request itself and exposes no way to reach its headers, and
			// hooks run before the request is logged, so the sanitizer still
			// sees everything.
			client.WithCallerBeforeDo(func(_ context.Context, req *http.Request) {
				req.Header.Set("Accept", "application/json")
			}),
		),
	}
}

// oauthConfig assembles the library config for this instance.
//
// Built per call rather than held on the struct only because it is cheap and
// keeps the endpoint resolution in one place; unlike the OIDC gateway there is
// no late-resolving discovery to wait for.
func (c *Client) oauthConfig() oauth2.Config {
	return oauth2.Config{
		ClientID:     c.provider.ClientID,
		ClientSecret: c.provider.ClientSecret,
		RedirectURL:  c.provider.RedirectURI,
		Scopes:       c.vendor.DefaultScopes(),
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.provider.AuthorizeURL,
			TokenURL: c.provider.TokenURL,
		},
	}
}

// AuthCodeURL builds the provider redirect /start sends the browser to.
//
// It returns an error to satisfy the DanceGateway contract the OIDC gateway
// shares, where the URL genuinely can fail to build because the endpoint comes
// from a discovery document that may not have answered. Here it cannot fail:
// the endpoints are configuration, validated at startup.
//
// Only the CHALLENGE goes out; the verifier stays in a cookie, which is what
// stops an intercepted authorization code from being redeemable.
func (c *Client) AuthCodeURL(_ context.Context, state, verifier string) (string, error) {
	cfg := c.oauthConfig()

	return cfg.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
	), nil
}

// Exchange trades an authorization code for the vendor's access token.
//
// The string it returns is an opaque access token, not an id_token -- the one
// place plain OAuth 2.0 diverges from OIDC at this layer. The DanceGateway
// contract calls it a credential precisely so both can satisfy it: what the
// string MEANS belongs to the provider that will consume it, and a provider is
// only ever handed its own gateway's output.
//
// The Accept header this endpoint requires is carried by tokenHTTPC; see
// NewClient for why it is a header and why it is not on the shared client.
func (c *Client) Exchange(ctx context.Context, code, codeVerifier string) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "gateway.OAuth2.Exchange",
		xfield.String("provider", c.provider.DisplayName))
	defer span.End()

	// oauth2 reads its HTTP client from the context, which is the only way to
	// reach the request it builds internally. Handing it an xhttp client keeps
	// this call under the same timeout and the same log-redaction policy as
	// every other outbound request.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.tokenHTTPC)

	cfg := c.oauthConfig()

	token, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return "", fmt.Errorf("%w: %w", apperr.ErrOAuthExchangeFailed, err)
	}

	// A successful exchange carrying no token is not a success with a missing
	// field: there is nothing to authenticate with, and returning "" would push
	// a confusing failure into the provider instead of reporting it here.
	if token.AccessToken == "" {
		return "", fmt.Errorf("%w: token response carried no access_token", apperr.ErrOAuthExchangeFailed)
	}

	return token.AccessToken, nil
}

// FetchIdentity resolves the account behind an access token.
//
// The reads themselves belong to the vendor; what belongs here is the budget
// that binds them together. A vendor cannot widen it: context.WithTimeout only
// ever tightens a deadline.
func (c *Client) FetchIdentity(ctx context.Context, accessToken string) (*entity.OAuth2Identity, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "gateway.OAuth2.FetchIdentity",
		xfield.String("provider", c.provider.DisplayName))
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, totalBudget)
	defer cancel()

	return c.vendor.ResolveIdentity(ctx, accessToken)
}

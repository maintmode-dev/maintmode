// Package googleoauth is the gateway to Google's OAuth token endpoint: one
// exchange per sign-in trades an authorization code for an id_token.
//
// It exists because RUK-291 made this backend a confidential OAuth client. The
// verification half of Google sign-in lives in services/authmethod/googleoauth
// and talks to nobody — it checks an id_token offline against Google's JWKS.
// This package is the other half, and the only one that needs the client secret.
//
// The exchange itself is golang.org/x/oauth2 rather than a hand-rolled POST:
// the grant type, the form encoding, the RFC 6749 error shape and the PKCE
// parameter are all its business, and reimplementing them buys nothing. What
// this package adds is the transport — oauth2 takes its *http.Client from the
// context, so the project's xhttp client goes in there and the exchange inherits
// the same timeout and log-redaction policy as every other outbound call.
package googleoauth

import (
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/ruko1202/xhttp/client"

	"github.com/ruko1202/maintmode/internal/config"
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

// Client talks to Google's token endpoint.
type Client struct {
	cfg   oauth2.Config
	httpc *http.Client
}

// NewClient builds the token-exchange client. The request timeout is the
// gateway's own business, not the caller's — see exchangeTimeout.
func NewClient(cfg config.GoogleOauthProvider) *Client {
	return &Client{
		cfg: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURI,
			Endpoint: oauth2.Endpoint{
				// Both come from config, with no in-code default. An endpoint
				// baked into the binary is one an operator cannot see when they
				// need to know where their sign-ins are going, and it would let
				// a stand run against Google while its config says otherwise.
				AuthURL:  cfg.AuthURL,
				TokenURL: cfg.TokenURL,
			},
			Scopes: []string{"openid", "email", "profile"},
		},
		httpc: client.NewClient(
			client.WithTimeout(exchangeTimeout),
			client.WithSanitizer(xsanitize.New()),
		),
	}
}

// AuthCodeURL builds the provider redirect /start sends the browser to.
//
// It lives here rather than in the handler because oauth2.Config already holds
// the client id, the redirect URI and the endpoint: assembling the same URL by
// hand in the API layer meant a second copy of all three, plus the response_type
// and challenge-method literals the library sets itself.
//
// Only the CHALLENGE goes out; the verifier stays in Valkey, which is what stops
// an intercepted authorization code from being redeemable.
func (c *Client) AuthCodeURL(state, verifier string) string {
	return c.cfg.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
	)
}

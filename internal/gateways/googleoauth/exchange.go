package googleoauth

import (
	"context"
	"fmt"

	"github.com/ruko1202/xlog"
	"golang.org/x/oauth2"

	"github.com/ruko1202/maintmode/internal/apperr"
)

// Exchange trades an authorization code for the id_token Google minted with it.
//
// The client secret and the PKCE verifier both travel in the request, which is
// what makes this a confidential-client exchange: possession of the code alone
// is not enough to redeem it. Only the id_token is returned — Google's own
// access token is for calling Google's APIs, which this service never does, so
// it is deliberately never read out of the response.
func (c *Client) Exchange(ctx context.Context, code, codeVerifier string) (string, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "gateway.GoogleOAuth.Exchange")
	defer span.End()

	// oauth2 reads its HTTP client from the context. Handing it the project's
	// xhttp client is what keeps this call under the same timeout and the same
	// log-redaction policy as every other outbound request; the library's own
	// default would be http.DefaultClient, which has neither.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpc)

	token, err := c.cfg.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return "", fmt.Errorf("%w: %w", apperr.ErrOAuthExchangeFailed, err)
	}

	// The id_token is an extra field rather than part of oauth2.Token: the
	// library models OAuth 2.0, and an id_token is OIDC on top of it.
	idToken, _ := token.Extra("id_token").(string)

	// A successful exchange carrying no id_token is not a success with a missing
	// field: there is nothing to verify, and returning "" here would push a
	// confusing failure into the verifier instead of reporting it where it
	// happened.
	if idToken == "" {
		return "", fmt.Errorf("%w: token response carried no id_token", apperr.ErrOAuthExchangeFailed)
	}

	return idToken, nil
}

package authmethod

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

type noopGateway struct{}

func (noopGateway) AuthCodeURL(context.Context, string, string) (string, error) { return "", nil }
func (noopGateway) Exchange(context.Context, string, string) (string, error)    { return "", nil }

// The Secure flag is aggregated over the providers that can dance, and the
// aggregation leans one way on purpose: a production cookie must never lose
// Secure because some other provider is plain http, while the reverse mistake
// costs a local sign-in and only where http is configured at all.
//
// It matters more now than it did in config: with no providers the answer is
// "secure", and that is exactly the state every stand is in right after the
// config section goes away.
func TestSnapshot_DanceCookieSecure(t *testing.T) {
	t.Parallel()

	const (
		https = "https://host/callback"
		plain = "http://localhost:3000/callback"
	)

	// Real URLs rather than a precomputed flag: the scheme is now read where the
	// aggregate is computed, so a test handing over the answer would stop
	// covering the parse.
	danceable := func(id, redirectURI string) providerInput {
		return providerInput{
			ID:          entity.AuthMethod(id),
			Gateway:     noopGateway{},
			RedirectURI: redirectURI,
		}
	}

	tests := map[string]struct {
		providers []providerInput
		want      bool
	}{
		"no providers at all is secure": {
			providers: nil,
			want:      true,
		},
		"a single https provider is secure": {
			providers: []providerInput{danceable("a", https)},
			want:      true,
		},
		"a single plain-http provider is not": {
			providers: []providerInput{danceable("a", plain)},
			want:      false,
		},
		"one https provider keeps Secure on for everyone": {
			providers: []providerInput{danceable("a", plain), danceable("b", https)},
			want:      true,
		},
		"every provider plain drops it": {
			providers: []providerInput{danceable("a", plain), danceable("b", plain)},
			want:      false,
		},
		"a plain provider that cannot dance does not count": {
			providers: []providerInput{{ID: "a", RedirectURI: plain}},
			want:      true,
		},
		"an unreadable redirect_uri counts as https": {
			providers: []providerInput{danceable("a", "://not a url")},
			want:      true,
		},
	}

	for name, tc := range tests {
		require.Equalf(t, tc.want,
			newSnapshot(nil, tc.providers).danceCookieSecure, "%s", name)
	}
}

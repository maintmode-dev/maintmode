package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
)

// TestValidateInstanceKey covers the rules on the map key.
//
// The character set is deliberately unconstrained -- see ValidateInstanceKey for
// why the pattern that used to be here was removed -- so what is left to pin is
// the reserved list, which exists for reasons that do not go away: an instance
// answering to stub or bootstrap would inherit privileges those methods gate
// behind a secret, and one keyed email_otp would emit a duplicate id in
// /auth/providers.
func TestValidateInstanceKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		key     string
		wantErr string
	}{
		{name: "a plain lowercase name", key: "google"},
		{name: "digits and underscores", key: "acme_sso2"},
		// Unconstrained means unconstrained: these are odd but harmless, and
		// pinning them keeps a charset rule from creeping back in unnoticed.
		{name: "uppercase", key: "Acme"},
		{name: "a dot", key: "acme.sso"},
		{
			name:    "empty",
			key:     "",
			wantErr: "must not be empty",
		},
		{
			// The stub verifies nothing; an instance keyed for it would be
			// substituted for every method on a use_stub stand.
			name:    "the stub name is reserved",
			key:     "stub",
			wantErr: "reserved",
		},
		{
			// Break-glass resolves its identity by configured email and skips
			// the seats cap.
			name:    "the bootstrap name is reserved",
			key:     "bootstrap",
			wantErr: "reserved",
		},
		{
			name:    "the unknown sentinel is reserved",
			key:     "unknown",
			wantErr: "reserved",
		},
		{
			// Would produce two entries with the same id in GET /auth/providers.
			name:    "an id the providers endpoint already emits is reserved",
			key:     "email_otp",
			wantErr: "reserved",
		},
		{
			name:    "the other providers id is reserved",
			key:     "email_password",
			wantErr: "reserved",
		},
		// github and email are deliberately NOT reserved: their constants exist
		// for follow-up work that has to be able to use the name.
		{name: "github is available", key: "github"},
		{name: "email is available", key: "email"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := config.ValidateInstanceKey(tc.key)
			if tc.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestDanceCookieSecure pins the aggregation over N instances. The direction
// matters more than the edge cases: a production cookie must never lose Secure
// because some other instance is plain.
func TestDanceCookieSecure(t *testing.T) {
	t.Parallel()

	// Only dance-capable instances count, so every fixture carries a secret.
	danceable := func(redirectURI string) config.OIDCProvider {
		return config.OIDCProvider{ClientSecret: "secret", RedirectURI: redirectURI}
	}

	cases := []struct {
		name      string
		instances map[string]config.OIDCProvider
		want      bool
	}{
		{
			name: "no instances fails safe",
			want: true,
		},
		{
			name:      "a single https instance",
			instances: map[string]config.OIDCProvider{"google": danceable("https://example.com/callback")},
			want:      true,
		},
		{
			// The only reason this flag is conditional at all.
			name:      "a single plain http localhost instance",
			instances: map[string]config.OIDCProvider{"google": danceable("http://localhost:9000/callback")},
			want:      false,
		},
		{
			name: "all http",
			instances: map[string]config.OIDCProvider{
				"a": danceable("http://localhost:9000/a"),
				"b": danceable("http://localhost:9000/b"),
			},
			want: false,
		},
		{
			// The load-bearing case: one plain instance must not strip Secure
			// from the production cookie.
			name: "mixed schemes keep Secure",
			instances: map[string]config.OIDCProvider{
				"local": danceable("http://localhost:9000/a"),
				"prod":  danceable("https://example.com/b"),
			},
			want: true,
		},
		{
			name:      "an unparseable redirect_uri fails safe",
			instances: map[string]config.OIDCProvider{"google": danceable("://not-a-url")},
			want:      true,
		},
		{
			// A BFF-only instance has no dance and no redirect URI, so it must
			// not drag the flag either way.
			name: "an instance with no dance is ignored",
			instances: map[string]config.OIDCProvider{
				"bff":  {},
				"dial": danceable("http://localhost:9000/a"),
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			providers := config.OauthProviders{OIDC: tc.instances}
			require.Equal(t, tc.want, providers.DanceCookieSecure())
		})
	}
}

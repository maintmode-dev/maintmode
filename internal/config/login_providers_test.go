package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// The whole path a provider's secret takes: a flat entry read from a real file,
// every <secret:KEY> reference moved out of the settings, and resolved
// from the secrets file. Read from a file rather than built in Go because the
// flat shape is viper's decoding (squash, remain) as much as ours.
func TestLoadProviders_MovesAndResolvesSecrets(t *testing.T) {
	t.Parallel()

	cfg := readProvidersConfig(t, `
oauth_providers:
  providers:
    github:
      display_name: GitHub
      authorize_url: https://github.com/login/oauth/authorize
      managed_by: config
      enabled: false
      client_id: <secret:login/github/client_id>
      redirect_uri: https://app.example/cb
      client_secret: <secret:login/github/client_secret>
`)

	require.NoError(t, cfg.OauthProviders.prepareProviders())
	require.NoError(t, cfg.applySecrets(secretStore{
		"login/github/client_secret": "s3cr3t", "login/github/client_id": "gh-client",
	}))

	github := cfg.OauthProviders.Providers["github"]
	require.Equal(t, ManagedByConfig, github.ManagedBy)
	require.Equal(t, lo.ToPtr(false), github.Enabled)
	require.Equal(t, "GitHub", github.DisplayName)
	// A reference is moved by its value, whatever the key: the config does not
	// know which fields a provider treats as secret.
	require.Equal(t, map[string]any{"redirect_uri": "https://app.example/cb"}, github.Settings)
	require.Equal(t, map[string]string{"client_secret": "s3cr3t", "client_id": "gh-client"}, github.Secrets)
}

func TestPrepareProviders_Refuses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider LoginProvider
		wantErr  string
	}{
		{
			// No default: which side owns a provider is always written down, so
			// nothing falls into a mode by omission.
			name:     "managed_by missing",
			provider: LoginProvider{LoginFacts: LoginFacts{DisplayName: "Google"}},
			wantErr:  "oauth_providers.providers.google: managed_by must be config or ui",
		},
		{
			name:     "managed_by misspelled",
			provider: LoginProvider{ManagedEntry: ManagedEntry{ManagedBy: "confg"}},
			wantErr:  "oauth_providers.providers.google: managed_by must be config or ui",
		},
		{
			// A provider the file owns must say whether it is on.
			name: "managed by config without enabled",
			provider: LoginProvider{ManagedEntry: ManagedEntry{
				ManagedBy: ManagedByConfig, Settings: map[string]any{"client_id": "c"},
			}},
			wantErr: "oauth_providers.providers.google: enabled must be set when managed_by is config",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			providers := OauthProviders{Providers: LoginProviders{"google": tc.provider}}

			err := providers.prepareProviders()

			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestPrepareProviders_Accepts(t *testing.T) {
	t.Parallel()

	providers := OauthProviders{Providers: LoginProviders{
		// Facts for a provider an admin creates in the UI. What else the entry
		// carries is ignored, secrets included: a leftover reference must not
		// fail startup, and a handover to the UI is one changed word.
		"google": {
			LoginFacts: LoginFacts{DisplayName: "Google", IssuerURL: "https://accounts.google.com"},
			ManagedEntry: ManagedEntry{
				ManagedBy: ManagedByUI,
				Enabled:   lo.ToPtr(true),
				Settings:  map[string]any{"client_id": "c", "client_secret": "<secret:login/google/client_secret>"},
			},
		},
		// Pinned off by the file: no credentials needed for a provider that is
		// never served.
		"github": {
			LoginFacts:   LoginFacts{DisplayName: "GitHub"},
			ManagedEntry: ManagedEntry{ManagedBy: ManagedByConfig, Enabled: lo.ToPtr(false)},
		},
	}}

	require.NoError(t, providers.prepareProviders())
	require.Nil(t, providers.Providers["google"].Secrets, "a ui entry's secrets are dropped")
	require.Nil(t, providers.Providers["google"].Settings, "a ui entry's settings are dropped")
	require.Equal(t, "Google", providers.Providers["google"].DisplayName, "a ui entry keeps its facts")
}

func readProvidersConfig(t *testing.T, yaml string) *AppConfig {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	cfg, err := readConfig(path)
	require.NoError(t, err)

	return cfg
}

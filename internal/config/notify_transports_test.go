package config

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

// A notify transport takes the same path a login provider's secret does: a
// flat entry read from a real file, its references moved out of the settings
// and resolved from the secrets file.
func TestLoadTransports_MovesAndResolvesSecrets(t *testing.T) {
	t.Parallel()

	cfg := readProvidersConfig(t, `
notify_transport:
  transports:
    slack:
      managed_by: config
      enabled: true
      api_url: https://slack.example/api
      bot_token: <secret:notify/slack/bot_token>
`)

	require.NoError(t, cfg.NotifyTransport.prepareTransports())
	require.NoError(t, cfg.applySecrets(secretStore{"notify/slack/bot_token": "tok"}))

	slack := cfg.NotifyTransport.Transports["slack"]
	require.Equal(t, ManagedByConfig, slack.ManagedBy)
	require.Equal(t, lo.ToPtr(true), slack.Enabled)
	require.Equal(t, map[string]any{"api_url": "https://slack.example/api"}, slack.Settings)
	require.Equal(t, map[string]string{"bot_token": "tok"}, slack.Secrets)
}

// A ui entry is left to the UI whole, so a reference it still carries must not
// be looked up: the secrets file no longer having the key is the normal state
// after a handover. Driven through applySecrets, because that is where a
// leftover reference used to fail -- the resolver walks into map[string]any.
func TestLoadEntries_UIEntryLeftoverReferenceDoesNotFailStartup(t *testing.T) {
	t.Parallel()

	cfg := readProvidersConfig(t, `
oauth_providers:
  providers:
    google:
      display_name: Google
      managed_by: ui
      client_secret: <secret:login/google/client_secret>
notify_transport:
  transports:
    telegram:
      managed_by: ui
      bot_token: <secret:notify/telegram/bot_token>
`)

	require.NoError(t, cfg.OauthProviders.prepareProviders())
	require.NoError(t, cfg.NotifyTransport.prepareTransports())
	require.NoError(t, cfg.applySecrets(secretStore{}))

	require.Nil(t, cfg.NotifyTransport.Transports["telegram"].Settings)
	require.Nil(t, cfg.OauthProviders.Providers["google"].Settings)
}

func TestPrepareTransports_Refuses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		entry   NotifyTransportEntry
		wantErr string
	}{
		{
			name:    "managed_by missing",
			entry:   NotifyTransportEntry{Enabled: lo.ToPtr(true)},
			wantErr: "notify_transport.transports.slack: managed_by must be config or ui",
		},
		{
			name:    "managed_by misspelled",
			entry:   NotifyTransportEntry{ManagedBy: "UI"},
			wantErr: "notify_transport.transports.slack: managed_by must be config or ui",
		},
		{
			name:    "managed by config without enabled",
			entry:   NotifyTransportEntry{ManagedBy: ManagedByConfig, Settings: map[string]any{"api_url": "u"}},
			wantErr: "notify_transport.transports.slack: enabled must be set when managed_by is config",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			transports := NotifyTransportConfig{Transports: NotifyTransportEntries{"slack": tc.entry}}

			require.ErrorContains(t, transports.prepareTransports(), tc.wantErr)
		})
	}
}

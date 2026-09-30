package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// Serial for the same reason as provision_test.go: Provision releases every
// provisioned row of a category its config does not declare, within the names
// its registry holds.

// declaredTransport is a valid enabled notify entry with its secret already
// moved out of the settings, the way the config loader leaves it.
func declaredTransport(settings map[string]any, secrets map[string]string) config.NotifyTransportEntry {
	return config.NotifyTransportEntry{
		ManagedBy: config.ManagedByConfig,
		Enabled:   lo.ToPtr(true),
		Settings:  settings,
		Secrets:   secrets,
	}
}

func TestProvision_InsertsDeclaredTransports(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	svc, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{
		kinds.slack: declaredTransport(
			map[string]any{"api_url": "https://slack.example/api"},
			map[string]string{"bot_token": "slack-token"}),
		kinds.telegram: declaredTransport(nil, map[string]string{"bot_token": "tg-token"}),
		// An unauthenticated relay: email's password is optional, so the entry
		// needs no secret at all.
		kinds.email: declaredTransport(map[string]any{
			"host": "relay.internal", "port": 25, "from": "maint@example.com", "tls_policy": "none",
		}, nil),
	})

	require.NoError(t, svc.Provision(ctx))

	for name, token := range map[string]string{kinds.slack: "slack-token", kinds.telegram: "tg-token", kinds.email: ""} {
		row, found := readRow(ctx, t, kinds.notify, name)
		require.True(t, found, name)
		require.True(t, row.Provisioned, name)
		require.True(t, row.Enabled, name)
		require.JSONEq(t, `{}`, row.Secrets, "%s: no secret is stored", name)
		if token != "" {
			require.NotContains(t, row.Config, token, "%s: the secret stays out of the config", name)
		}
	}

	slack, _ := readRow(ctx, t, kinds.notify, kinds.slack)
	require.Contains(t, slack.Config, "https://slack.example/api")
}

func TestProvision_InsertsAnAuthenticatedEmailTransport(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	svc, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{
		kinds.email: declaredTransport(map[string]any{
			"host": "smtp.example.com", "port": 587, "from": "maint@example.com",
			"username": "maint", "tls_policy": "mandatory",
		}, map[string]string{"password": "smtp-password"}),
	})

	require.NoError(t, svc.Provision(ctx))

	row, found := readRow(ctx, t, kinds.notify, kinds.email)
	require.True(t, found)
	require.True(t, row.Provisioned)
	require.JSONEq(t, `{}`, row.Secrets)
	require.NotContains(t, row.Config, "smtp-password")
}

// Pinned off: provisioned and disabled, with no credentials asked for.
func TestProvision_PinnedOffTransportNeedsNoCredentials(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	svc, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{
		kinds.slack: {ManagedBy: config.ManagedByConfig, Enabled: lo.ToPtr(false)},
	})

	require.NoError(t, svc.Provision(ctx))

	row, found := readRow(ctx, t, kinds.notify, kinds.slack)
	require.True(t, found)
	require.True(t, row.Provisioned)
	require.False(t, row.Enabled)
}

func TestProvision_TakesOverAnAdminTransport(t *testing.T) {
	ctx := context.Background()
	adminSvc, kinds, _ := initService(t)
	_, err := adminSvc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind: kinds.notify, Name: kinds.slack, Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{}`),
		Secrets: json.RawMessage(`{"bot_token":"admin-typed"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	before, _ := readRow(ctx, t, kinds.notify, kinds.slack)
	require.NotEqual(t, `{}`, before.Secrets)

	svc, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{
		kinds.slack: declaredTransport(nil, map[string]string{"bot_token": "cfg-token"}),
	})
	require.NoError(t, svc.Provision(ctx))

	after, _ := readRow(ctx, t, kinds.notify, kinds.slack)
	require.Equal(t, before.ID, after.ID)
	require.True(t, after.Provisioned)
	require.JSONEq(t, `{}`, after.Secrets, "the admin-entered token is wiped")
	require.Nil(t, after.UpdatedBy)
}

// Each category releases only its own rows: dropping every transport from the
// config leaves the declared provider provisioned, and dropping the provider
// leaves the transport.
func TestProvision_ReleasesPerCategory(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	transport := config.NotifyTransportEntries{
		kinds.slack: declaredTransport(nil, map[string]string{"bot_token": "cfg-token"}),
	}
	provider := config.LoginProviders{kinds.oidc: declared("cfg-client")}

	both, _ := newServiceDeclaring(t, kinds, nil, provider, transport)
	require.NoError(t, both.Provision(ctx))

	loginOnly, _ := newServiceDeclaring(t, kinds, nil, provider, nil)
	require.NoError(t, loginOnly.Provision(ctx))

	slack, _ := readRow(ctx, t, kinds.notify, kinds.slack)
	require.False(t, slack.Provisioned, "the dropped transport is released")
	require.False(t, slack.Enabled)
	login, _ := readRow(ctx, t, kinds.login, kinds.oidc)
	require.True(t, login.Provisioned, "the provider is not the notify section's to release")

	require.NoError(t, both.Provision(ctx))
	notifyOnly, _ := newServiceDeclaring(t, kinds, nil, nil, transport)
	require.NoError(t, notifyOnly.Provision(ctx))

	slack, _ = readRow(ctx, t, kinds.notify, kinds.slack)
	require.True(t, slack.Provisioned, "the transport is not the login section's to release")
	login, _ = readRow(ctx, t, kinds.login, kinds.oidc)
	require.False(t, login.Provisioned, "the dropped provider is released")
}

// Handing a transport back to the UI is one changed word: the section stays,
// managed_by becomes ui. The loader keeps a leftover enabled line, so the
// entry must be skipped by its mode, not by what it happens to carry.
func TestProvision_ReleasesATransportSwitchedToUI(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	cfgOwned, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{
		kinds.slack: declaredTransport(nil, map[string]string{"bot_token": "cfg-token"}),
	})
	require.NoError(t, cfgOwned.Provision(ctx))

	uiOwned, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{
		kinds.slack: {ManagedBy: config.ManagedByUI, Enabled: lo.ToPtr(true)},
	})
	require.NoError(t, uiOwned.Provision(ctx))

	row, found := readRow(ctx, t, kinds.notify, kinds.slack)
	require.True(t, found)
	require.False(t, row.Provisioned, "the ui entry hands the row back")
	require.False(t, row.Enabled)
}

func TestProvision_InvalidTransportChangesNothing(t *testing.T) {
	cases := []struct {
		name string
		decl func(kinds testKinds) config.NotifyTransportEntries
	}{
		{
			name: "unknown name",
			decl: func(kinds testKinds) config.NotifyTransportEntries {
				return config.NotifyTransportEntries{
					kinds.slack:  declaredTransport(nil, map[string]string{"bot_token": "t"}),
					"no-such-tx": declaredTransport(nil, nil),
				}
			},
		},
		{
			// A login provider's name is not a transport, whatever the section.
			name: "login name in the notify section",
			decl: func(kinds testKinds) config.NotifyTransportEntries {
				return config.NotifyTransportEntries{
					kinds.slack: declaredTransport(nil, map[string]string{"bot_token": "t"}),
					kinds.oidc:  declaredTransport(nil, nil),
				}
			},
		},
		{
			name: "literal bot_token",
			decl: func(kinds testKinds) config.NotifyTransportEntries {
				return config.NotifyTransportEntries{
					kinds.slack: declaredTransport(map[string]any{"bot_token": "from-the-secrets-file"}, nil),
				}
			},
		},
		{
			// A pinned-off entry never reaches Validate, and its config is
			// still written into the row.
			name: "literal bot_token on a pinned-off entry",
			decl: func(kinds testKinds) config.NotifyTransportEntries {
				return config.NotifyTransportEntries{kinds.slack: {
					ManagedBy: config.ManagedByConfig, Enabled: lo.ToPtr(false),
					Settings: map[string]any{"bot_token": "from-the-secrets-file"},
				}}
			},
		},
		{
			name: "missing bot_token",
			decl: func(kinds testKinds) config.NotifyTransportEntries {
				return config.NotifyTransportEntries{kinds.slack: declaredTransport(nil, nil)}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			_, kinds, _ := initService(t)
			svc, _ := newServiceDeclaring(t, kinds, nil, nil, tc.decl(kinds))

			err := svc.Provision(ctx)

			require.Error(t, err)
			require.Contains(t, err.Error(), "provisioned notify/")
			require.NotContains(t, err.Error(), "from-the-secrets-file")
			_, found := readRow(ctx, t, kinds.notify, kinds.slack)
			require.False(t, found, "one bad entry writes no row")
		})
	}
}

// A literal secret is refused as a validation error naming the key.
func TestProvision_LiteralTransportSecretNamesTheKey(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	svc, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{
		kinds.telegram: declaredTransport(map[string]any{"bot_token": "123:abc"}, nil),
	})

	err := svc.Provision(ctx)

	require.ErrorIs(t, err, apperr.ErrValidation)
	require.ErrorContains(t, err, "bot_token must be a <secret:KEY> reference")
	require.NotContains(t, err.Error(), "123:abc")
}

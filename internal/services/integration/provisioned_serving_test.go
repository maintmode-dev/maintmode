package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	integrationsvc "github.com/ruko1202/maintmode/internal/services/integration"
)

func listedLogin(ctx context.Context, t *testing.T, svc *integrationsvc.Service, name string) entity.ConfiguredProvider {
	t.Helper()

	providers, err := svc.ListLoginProviders(ctx, integrationkinds.CategoryLogin)
	require.NoError(t, err)

	provider, found := lo.Find(providers, func(p entity.ConfiguredProvider) bool { return p.Name == name })
	require.True(t, found, "provider %q must be listed", name)

	return provider
}

// A provisioned provider is served from this replica's own config: settings,
// secret and enabled all come from memory. The row's config is only a display
// copy, written by whichever replica started last -- serving from it would pair
// that replica's client_id with this one's secret.
//
// Not parallel: it provisions (see provision_test.go).
//
// Proven by mutation: dropping the memory branch turns this red.
func TestListLoginProviders_ServesAProvisionedRowFromMemory(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	svc, _ := newServiceFor(t, kinds, nil, config.LoginProviders{kinds.oidc: declared("cfg-client")})
	require.NoError(t, svc.Provision(ctx))

	_, err := db.ExecContext(ctx,
		`UPDATE integration_settings SET config = jsonb_set(config, '{client_id}', '"another-replica"'), enabled = false
		 WHERE kind = $1 AND name = $2`, kinds.login, kinds.oidc)
	require.NoError(t, err)

	provider := listedLogin(ctx, t, svc, kinds.oidc)

	require.False(t, provider.Unreadable)
	require.True(t, provider.Enabled, "enabled comes from memory too")
	settings, ok := provider.Settings.(integrationkinds.OIDCSettings)
	require.True(t, ok)
	require.Equal(t, "cfg-client", settings.ClientID)
	require.Equal(t, "from-the-secrets-file", settings.ClientSecret)
}

// A released row has no secret. Opening it can only fail, and reporting that
// as unreadable -- with an error log every reload tick on every replica --
// would cry wolf about a provider that is simply off.
func TestListLoginProviders_DisabledRowWithoutSecretIsNotUnreadable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	createDisabledLogin(ctx, t, svc, kinds)

	_, err := db.ExecContext(ctx,
		`UPDATE integration_settings SET secrets = '{}' WHERE kind = $1 AND name = $2`, kinds.login, kinds.oidc)
	require.NoError(t, err)

	provider := listedLogin(ctx, t, svc, kinds.oidc)

	require.False(t, provider.Unreadable)
	require.False(t, provider.Enabled)
}

// The short-circuit is for rows with NO secret. A disabled row whose secret no
// longer opens -- a lost DEK or KEK -- keeps reporting unreadable, which is the
// only warning an admin gets before enabling it.
func TestListLoginProviders_DisabledRowWithBrokenSecretStaysUnreadable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	createDisabledLogin(ctx, t, svc, kinds)

	_, err := db.ExecContext(ctx,
		`UPDATE integration_settings SET secrets = '{"client_secret":"AAAAAAAAAAAAAAAA"}' WHERE kind = $1 AND name = $2`,
		kinds.login, kinds.oidc)
	require.NoError(t, err)

	provider := listedLogin(ctx, t, svc, kinds.oidc)

	require.True(t, provider.Unreadable)
}

func createDisabledLogin(ctx context.Context, t *testing.T, svc *integrationsvc.Service, kinds testKinds) {
	t.Helper()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind: kinds.login, Name: kinds.oidc, Enabled: lo.ToPtr(false),
		Config:  presetConfig("corp-client"),
		Secrets: json.RawMessage(`{"client_secret":"corp-secret"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err)
}

// The documented way back from a release: PATCH the secret in and enable it.
// A released row carries no secret but kept its DEK, so the admin path must
// seal the new secret under it and the provider must open again.
func TestProvision_ReleasedRowIsRecoveredByAPatch(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	first, _ := newServiceFor(t, kinds, nil, config.LoginProviders{kinds.oidc: declared("cfg-client")})
	require.NoError(t, first.Provision(ctx))

	next, _ := newServiceFor(t, kinds, nil, config.LoginProviders{})
	require.NoError(t, next.Provision(ctx))

	updated, err := next.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind: kinds.login, Name: kinds.oidc,
		Enabled: lo.ToPtr(true),
		Secrets: json.RawMessage(`{"client_secret":"re-entered"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	require.False(t, updated.Provisioned)
	require.True(t, updated.Enabled)

	provider := listedLogin(ctx, t, next, kinds.oidc)
	require.False(t, provider.Unreadable)
	require.True(t, provider.Enabled)
	settings, ok := provider.Settings.(integrationkinds.OIDCSettings)
	require.True(t, ok)
	require.Equal(t, "cfg-client", settings.ClientID)
	require.Equal(t, "re-entered", settings.ClientSecret)
}

// managed_by: config with enabled: false pins a provider off: the row is the
// file's, so the UI cannot switch it back on. It is never served, so it needs
// no credentials -- demanding them would push an operator towards handing the
// provider to the UI just to switch it off.
func TestProvision_DisabledEntryNeedsNoCredentials(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	svc, _ := newServiceFor(t, kinds, nil, config.LoginProviders{kinds.oidc: {
		LoginFacts: config.LoginFacts{DisplayName: "Test IdP", IssuerURL: testPresetIssuer},
		ManagedBy:  config.ManagedByConfig,
		Enabled:    lo.ToPtr(false),
	}})

	require.NoError(t, svc.Provision(ctx))

	row, found := readRow(ctx, t, kinds.login, kinds.oidc)
	require.True(t, found)
	require.True(t, row.Provisioned, "the file owns it, so the admin API refuses to switch it on")
	require.False(t, row.Enabled)

	provider := listedLogin(ctx, t, svc, kinds.oidc)
	require.False(t, provider.Enabled)
	require.False(t, provider.Unreadable)
}

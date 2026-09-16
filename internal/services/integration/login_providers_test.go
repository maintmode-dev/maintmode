package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// ListLoginProviders is what the reloader calls to build the live login
// snapshot, and it is the one path in this service addressed by CATEGORY rather
// than by a system name.
//
// That distinction is the whole test. The registry keys implementations by
// name, so resolving one implementation for the whole call -- which worked
// while the category and the system were the same string -- now looks up
// "login", finds nothing, and returns an error before a single row is read.
// Sign-in would be dead with nothing but a periodic "reload failed" line.
//
// The reloader's own tests cannot catch it: their fakeRegistry replaces this
// method wholesale.
func TestListLoginProviders_ResolvesPerRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.oidc,
		Enabled: lo.ToPtr(true),
		Config:  presetConfig("client-a"),
		Secrets: json.RawMessage(`{"client_secret":"secret-a"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	// A delivery row in the same database, to prove the listing is scoped by
	// category rather than returning everything.
	_, err = svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.notify,
		Name:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://slack.test"}`),
		Secrets: json.RawMessage(`{"bot_token":"xoxb-1"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	providers, err := svc.ListLoginProviders(ctx, integrationkinds.CategoryLogin)
	require.NoError(t, err, "the category is not a registry key; the implementation resolves per row")

	found, ok := lo.Find(providers, func(p entity.ConfiguredProvider) bool { return p.Name == kinds.oidc })
	require.True(t, ok, "the stored login row must be listed")
	require.False(t, found.Unreadable, "its settings must open, which needs the row's own implementation")
	require.NotNil(t, found.Settings)

	require.False(t,
		lo.ContainsBy(providers, func(p entity.ConfiguredProvider) bool { return p.Name == kinds.slack }),
		"a delivery row must not appear in the login listing")
}

// The read path re-asserts the field rules, not just the write path.
//
// Those rules ARE the SSRF guard and the https requirement. A row can reach the
// table without them having run -- a restore from a backup predating the guard,
// a direct database write, a migration branch -- and without re-validation it
// would be loaded and served because it was once written, which is not the same
// as being valid now.
//
// The row here is tampered with directly in the database, which is exactly the
// route that bypasses Create and Update.
//
// redirect_uri is the field used, deliberately: issuer_url and client_id are
// AAD inputs, so moving either makes the secret undecryptable and the row is
// reported unreadable before validation is reached -- the AAD doing its own
// job. redirect_uri is security-relevant but NOT an AAD input, so it is the
// field that reaches this check.
func TestListLoginProviders_RevalidatesStoredSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.byoIssuer,
		Enabled: lo.ToPtr(true),
		Config:  oidcConfig("https://idp.corp.example", "client-a"),
		Secrets: json.RawMessage(`{"client_secret":"secret-a"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	// Break it behind the service's back. Create and Update both refuse an
	// empty redirect_uri; a direct write does not.
	_, err = db.ExecContext(ctx,
		`UPDATE integration_settings
		    SET config = jsonb_set(config::jsonb, '{redirect_uri}', '""')::json
		  WHERE name = $1`, kinds.byoIssuer)
	require.NoError(t, err)

	providers, err := svc.ListLoginProviders(ctx, integrationkinds.CategoryLogin)
	require.NoError(t, err, "one bad row must not fail the whole rebuild")

	found, ok := lo.Find(providers, func(p entity.ConfiguredProvider) bool {
		return p.Name == kinds.byoIssuer
	})
	require.True(t, ok, "the row is still listed, so an operator can see its state")
	// The FACT, not the reason: what reaches a caller is "this row cannot be
	// served", and why is in the log line this listing already wrote. A test
	// asserting the sentinel would be asserting a classification nothing
	// downstream reads.
	require.True(t, found.Unreadable,
		"a stored config the field rules would have refused must not be served")
	require.Nil(t, found.Settings)
}

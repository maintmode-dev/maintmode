package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// oidcConfig is a complete, valid provider config; only the fields the test
// varies are worth reading here.
// oidcConfig is what an operator sends for a `custom` provider: everything,
// issuer included, because no catalog entry speaks for that name.
//
// Tests that exercise a PRESET provider send presetConfig instead -- supplying
// issuer_url there is refused, which is the point of §6.5.
func oidcConfig(issuer, clientID string) json.RawMessage {
	return json.RawMessage(`{
		"display_name":"Provider",
		"issuer_url":"` + issuer + `",
		"client_id":"` + clientID + `",
		"redirect_uri":"https://app.example/auth/callback"
	}`)
}

// presetConfig is what an operator sends for a preset provider: the fields of
// their own OAuth client, and nothing the catalog owns.
func presetConfig(clientID string) json.RawMessage {
	return json.RawMessage(`{
		"client_id":"` + clientID + `",
		"redirect_uri":"https://app.example/auth/callback"
	}`)
}

// A login row is addressed by (category, name) like any other, and the pair is
// what the store keys on.
//
// This used to assert that two providers of one kind coexist -- the criterion of
// the previous iteration, when a login kind took any instance name. The closed
// set replaced that: there is one row per registered name, and a second one
// would need a name nothing implements. What survives is the addressing.
func TestCreate_LoginRowIsAddressedByCategoryAndName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	created, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.oidc,
		Enabled: lo.ToPtr(true),
		Config:  presetConfig("corp-client"),
		Secrets: json.RawMessage(`{"client_secret":"corp-secret"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	require.Equal(t, kinds.oidc, created.Name)
	require.Equal(t, kinds.login, created.Kind)

	got, err := svc.GetByKindName(ctx, kinds.login, kinds.oidc)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Contains(t, string(got.Config), testPresetIssuer,
		"the preset issuer is what the row carries, not whatever the caller sent")

	// The name alone is not the address: the same name under the wrong category
	// is a different row, and there is none.
	_, err = svc.GetByKindName(ctx, kinds.notify, kinds.oidc)
	require.ErrorIs(t, err, apperr.ErrIntegrationNotFound)
}

// The schema stopped enforcing this when UNIQUE (kind) went away, so the
// service has to, or "no multi-instance for Slack" becomes a comment.
func TestCreate_SecondInstanceOfSingleInstanceKindRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.notify,
		Name:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://slack.test"}`),
		Secrets: json.RawMessage(`{"bot_token":"xoxb-1"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	// Under the SAME name, so what is exercised is the count rather than the
	// name rule -- the schema would accept this row now that UNIQUE (kind) is
	// gone, and only the service stands between it and a duplicate.
	_, err = svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.notify,
		Name:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://slack2.test"}`),
		Secrets: json.RawMessage(`{"bot_token":"xoxb-2"}`),
		Actor:   actor,
	})
	require.ErrorIs(t, err, apperr.ErrIntegrationConflict,
		"a delivery kind stays single-instance even though the schema would now allow a second row")
}

// A login provider's secret is sealed against its issuer and client, so a row
// must open only under its own binding -- the property that stops an at-rest
// writer moving ciphertext between providers.
//
// It used to create two providers side by side, which the closed set no longer
// allows: the fixture registers one login entry, and a second would have to be
// a name nothing implements. The same property is proved by re-pointing one row
// instead, which is the move the binding actually defends against.
func TestCreate_ProviderSecretOpensOnlyUnderItsOwnBinding(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.oidc,
		Enabled: lo.ToPtr(true),
		Config:  presetConfig("google-client"),
		Secrets: json.RawMessage(`{"client_secret":"google-secret"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	settings, err := svc.Settings(ctx, kinds.login, kinds.oidc)
	require.NoError(t, err, "the row must decrypt under its own binding")
	oidcSettings, ok := settings.(integrationkinds.OIDCSettings)
	require.True(t, ok)
	require.Equal(t, "google-secret", oidcSettings.ClientSecret)

	// Move the issuer underneath the stored ciphertext, exactly as an at-rest
	// writer with no KEK would: the config is plaintext, the secret and dek_id
	// are carried forward untouched. The AAD binds the issuer, so the row must
	// stop opening rather than hand the secret to the new destination.
	_, err = db.ExecContext(ctx,
		`UPDATE integration_settings
		    SET config = jsonb_set(config::jsonb, '{issuer_url}', '"https://okta.corp.example"')::json
		  WHERE name = $1`, kinds.oidc)
	require.NoError(t, err)

	_, err = svc.Settings(ctx, kinds.login, kinds.oidc)
	require.ErrorIs(t, err, apperr.ErrIntegrationUnreadable,
		"a secret sealed against one issuer must not open against another")
}

// Re-pointing a provider people already sign in through is ALLOWED, and this
// pins that it stays allowed.
//
// It used to be refused, and the refusal was the wrong shape: an IdP migration
// (Keycloak to Okta, say) is something an admin is entitled to do, and there
// was no way to say "I know" -- the only path past the refusal was to unlink
// every account by hand, which is worse than what it prevented. Admin means
// admin.
//
// What still holds is the technical invariant next to it: the stored secret is
// sealed against the issuer and client id, so moving them without supplying the
// secret again is refused by checkAADBindingStable. That is not a question of
// authority but of whether the ciphertext survives -- which is why this test
// sends the new secret along with the new issuer.
//
// A PRESET provider is the other half of the rule and is covered elsewhere:
// `google` cannot be aimed anywhere, because applyPreset and enforcePreset own
// its issuer whatever the caller sends. This test uses the BYO name on purpose.
func TestUpdate_RepointingLinkedProviderAllowed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, mocks := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.byoIssuer,
		Enabled: lo.ToPtr(true),
		Config:  oidcConfig("https://idp.corp.example", "corp-client"),
		Secrets: json.RawMessage(`{"client_secret":"corp-secret"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	// Accounts now sign in through this provider.
	mocks.identities.linked = 4

	newSecret := "migrated-secret"
	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.byoIssuer,
		Config:  oidcConfig("https://okta.corp.example", "migrated-client"),
		Secrets: json.RawMessage(`{"client_secret":"` + newSecret + `"}`),
		Actor:   actor,
	})
	require.NoError(t, err,
		"an admin re-pointing a BYO provider is a migration, not an attack to block")

	// And it actually took effect, secret included -- a "success" that wrote
	// nothing would be the same bug wearing a different status code.
	settings, err := svc.Settings(ctx, kinds.login, kinds.byoIssuer)
	require.NoError(t, err)
	require.Equal(t, "https://okta.corp.example", settings.(integrationkinds.OIDCSettings).IssuerURL)
	require.Equal(t, newSecret, settings.(integrationkinds.OIDCSettings).ClientSecret,
		"the new secret must be readable under the new binding")
}

// The same edit on a provider nobody uses is ordinary maintenance.
func TestUpdate_RepointingUnusedProviderAllowed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.byoIssuer,
		Enabled: lo.ToPtr(true),
		Config:  oidcConfig("https://idp.corp.example", "client-1"),
		Secrets: json.RawMessage(`{"client_secret":"secret-1"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.byoIssuer,
		Config:  oidcConfig("https://idp.corp.example", "client-1"),
		Secrets: json.RawMessage(`{"client_secret":"secret-2"}`),
		Actor:   actor,
	})
	require.NoError(t, err, "fixing a typo before anyone has signed in must stay cheap")

	// And the re-sealed secret opens under the new binding.
	settings, err := svc.Settings(ctx, kinds.login, kinds.byoIssuer)
	require.NoError(t, err)
	require.Equal(t, "secret-2", settings.(integrationkinds.OIDCSettings).ClientSecret)
}

// A cosmetic issuer edit -- a trailing slash, a differently-cased host -- must
// leave the provider readable.
//
// The guard treats those as "not a change" and lets the update through without
// a new secret, which is right: discovery trims the slash and the host is
// case-insensitive, so it is the same IdP. But the seal has to agree. If the
// guard normalizes and the AAD does not, the stored ciphertext is carried
// forward under one value and reopened under another: the provider dies with
// ErrIntegrationUnreadable on a read nobody connects to this edit, and the
// plaintext is gone.
func TestUpdate_CosmeticIssuerEditKeepsTheSecretReadable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.byoIssuer,
		Enabled: lo.ToPtr(true),
		Config:  oidcConfig("https://idp.corp.example", "corp-client"),
		Secrets: json.RawMessage(`{"client_secret":"corp-secret"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	// Same IdP, written differently. No secret resupplied, because the operator
	// was told none was needed.
	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:   kinds.login,
		Name:   kinds.byoIssuer,
		Config: oidcConfig("https://IDP.corp.example/", "corp-client"),
		Actor:  actor,
	})
	require.NoError(t, err, "a cosmetic issuer edit must be allowed without resupplying the secret")

	settings, err := svc.Settings(ctx, kinds.login, kinds.byoIssuer)
	require.NoError(t, err,
		"the provider must still open: an edit the guard called harmless cannot be the one that kills it")
	require.Equal(t, "corp-secret", settings.(integrationkinds.OIDCSettings).ClientSecret)
}

// Deleting a provider people sign in through takes their identities with it,
// through the real Delete rather than the cascade predicate alone -- a cascade
// nothing calls is a cascade that does not exist, which this work has already
// demonstrated once, when removing a call from Update left the predicate's own
// tests green.
//
// It used to be REFUSED, and the refusal was unsatisfiable: it said "unlink
// them first", and the only unlink in the product is /me, performed by the
// account's owner and itself refused when it would remove their last sign-in
// method. An admin had no way to comply short of editing the database.
//
// Leaving the rows instead would be the worse of the three. provider is a bare
// string with no foreign key, so an orphan waits for the name to be created
// again against another IdP and then vouches for it against an account that
// predates it.
func TestDelete_ProviderWithLinkedAccountsCascades(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, mocks := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.oidc,
		Enabled: lo.ToPtr(true),
		Config:  presetConfig("corp-client"),
		Secrets: json.RawMessage(`{"client_secret":"corp-secret"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	// The row's own id, read before the delete: the cascade must address THIS
	// row, and asserting against a value the test did not fetch would pass just
	// as well against any other.
	created, err := svc.GetByKindName(ctx, kinds.login, kinds.oidc)
	require.NoError(t, err)

	mocks.identities.linked = 7

	require.NoError(t, svc.Delete(ctx, kinds.login, kinds.oidc, actor),
		"an admin removing a provider has decided something; there is no question left to ask")

	// Addressed by the registry row's ID. By category it would take every login
	// provider's rows; by name it would follow a name that can be re-created
	// against a different IdP, which is the whole reason the column changed.
	require.Equal(t, created.ID, mocks.identities.unlinkedFrom)
	require.Zero(t, mocks.identities.linked, "the identities must be gone, not merely counted")

	// And the row itself went with them.
	_, err = svc.GetByKindName(ctx, kinds.login, kinds.oidc)
	require.ErrorIs(t, err, apperr.ErrIntegrationNotFound)
}

// A DELIVERY row is not a login provider, and deleting one must not reach into
// user_identities at all. The cascade is scoped by category for that reason:
// unscoped, removing a Slack row named the same as an OIDC provider would take
// that provider's accounts with it.
func TestDelete_DeliveryRowDoesNotCascade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, mocks := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.notify,
		Name:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"default_channel":"#ops"}`),
		Secrets: json.RawMessage(`{"bot_token":"xoxb-1"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	mocks.identities.linked = 3

	require.NoError(t, svc.Delete(ctx, kinds.notify, kinds.slack, actor))
	require.Empty(t, mocks.identities.unlinkedFrom,
		"a notify row must not touch user_identities")
	require.EqualValues(t, 3, mocks.identities.linked, "nobody's sign-in may go with a Slack row")
}

// Removing a provider nobody uses is ordinary maintenance and must work.
func TestDelete_UnusedProviderRemoved(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.oidc,
		Enabled: lo.ToPtr(true),
		Config:  presetConfig("client-1"),
		Secrets: json.RawMessage(`{"client_secret":"secret-1"}`),
		Actor:   actor,
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, kinds.login, kinds.oidc, actor))

	_, err = svc.GetByKindName(ctx, kinds.login, kinds.oidc)
	require.ErrorIs(t, err, apperr.ErrIntegrationNotFound)
}

// The closed set is a set of PAIRS, and this is the half that is easy to lose.
//
// A wrong NAME is refused by the registry lookup anyway -- nothing is
// registered under it. A right name under the WRONG CATEGORY is not: it walks
// past the login guards, which key on the category, and past the preset rule,
// which keys on the name, and only falls over much later at a delivery lookup
// that finds nothing. Deleting the category comparison in Registry.admit leaves the
// rest of the suite green, so the assertion has to be made here.
func TestCreate_MismatchedCategoryAndSystemRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	create := func(category, name string) error {
		_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
			Kind:    category,
			Name:    name,
			Enabled: lo.ToPtr(true),
			Config:  json.RawMessage(`{"api_url":"https://slack.test"}`),
			Secrets: json.RawMessage(`{"bot_token":"xoxb-1"}`),
			Actor:   testActor(),
		})

		return err
	}

	// Both halves are registered and both are legal -- just not together.
	err := create(kinds.notify, kinds.oidc)
	require.ErrorIs(t, err, apperr.ErrValidation,
		"a login system under the notify category must be refused")
	require.ErrorContains(t, err, "login",
		"the refusal must say which category the system belongs to, "+
			"or it is indistinguishable from an unknown name")

	err = create(kinds.login, kinds.slack)
	require.ErrorIs(t, err, apperr.ErrValidation,
		"a delivery system under the login category must be refused")
	require.ErrorContains(t, err, "notify")

	// And the unknown-name refusal stays a DIFFERENT condition: it carries the
	// registry's own sentinel, so a caller (and the API's error mapping) can
	// tell "no such integration" from "wrong half".
	_, err = svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.notify,
		Name:    "nonexistent-" + xuuid.NewString(),
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://slack.test"}`),
		Secrets: json.RawMessage(`{"bot_token":"xoxb-1"}`),
		Actor:   testActor(),
	})
	require.ErrorIs(t, err, apperr.ErrUnknownIntegrationKind)
}

// Creating a provider under a name that identities already carry is ALLOWED,
// and this pins that it stays allowed rather than looking like an oversight.
//
// It was refused, and the refusal made sense while a name could come to mean a
// different IdP: re-create "google" pointed elsewhere and every account on that
// name signs in somewhere new, successfully, with nothing reporting a problem.
//
// The closed set removed that. There are two login names. `google` cannot be
// aimed anywhere -- applyPreset and enforcePreset own its issuer, on create and
// on update alike, which TestUpdate_PresetFieldCannotBeRewritten covers. And
// `custom` is an admin's to re-point by design; refusing to CREATE it bought
// nothing, since the same admin could edit the row instead.
//
// What is left is a stale row surviving its provider, which the delete cascade
// now prevents in the first place, and which RUK-303 will make impossible by
// making the column a foreign key.
func TestCreate_NameWithLinkedAccountsAllowed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, mocks := initService(t)

	// Identities exist for this name with no row backing it -- the state the
	// old guard refused.
	mocks.identities.linked = 12

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.oidc,
		Enabled: lo.ToPtr(true),
		Config:  presetConfig("corp-client"),
		Secrets: json.RawMessage(`{"client_secret":"corp-secret"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err, "a preset provider cannot be aimed elsewhere, so the name is safe to take")

	// And the row is the preset's, not the caller's idea of it.
	settings, err := svc.Settings(ctx, kinds.login, kinds.oidc)
	require.NoError(t, err)
	require.Equal(t, testPresetIssuer, settings.(integrationkinds.OIDCSettings).IssuerURL)
}

// The preset rule reaching the real Update path.
//
// enforcePreset is unit-tested next door, and that proves the rule is right --
// never that Update calls it. The bypass this guards was exactly that shape:
// create refused a caller-supplied issuer under a preset name, and Update, which
// replaces Config wholesale, accepted it one request later.
//
// This is now the ONLY thing keeping a preset provider pointed where it
// belongs. The linked-account refusal that used to sit beside it is gone --
// re-pointing is an admin's call -- so a preset name is no longer protected by
// "someone already signs in through it", and never was in the window right
// after an operator creates one. If this test goes green with enforcePreset
// unwired, `google` can be aimed at any issuer a caller names.
func TestUpdate_PresetFieldCannotBeRewritten(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, mocks := initService(t)
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

	// No accounts linked yet -- the window an operator is actually in.
	mocks.identities.linked = 0

	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.oidc,
		Config:  oidcConfig("https://okta.corp.example", "client-a"),
		Secrets: json.RawMessage(`{"client_secret":"secret-a"}`),
		Actor:   actor,
	})
	require.ErrorIs(t, err, apperr.ErrValidation,
		"rewriting a preset issuer must be refused on update, not only on create")

	// And the refusal is complete: the stored row keeps the catalog's issuer.
	settings, err := svc.Settings(ctx, kinds.login, kinds.oidc)
	require.NoError(t, err)
	require.Equal(t, testPresetIssuer, settings.(integrationkinds.OIDCSettings).IssuerURL,
		"a refused update must not have written anything")

	// An update that leaves the preset fields alone is ordinary maintenance and
	// must still go through -- otherwise this guard would freeze the whole row.
	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind: kinds.login,
		Name: kinds.oidc,
		Config: json.RawMessage(`{
			"issuer_url":"` + testPresetIssuer + `",
			"display_name":"Test IdP",
			"client_id":"client-b",
			"redirect_uri":"https://app.example/auth/callback"
		}`),
		Secrets: json.RawMessage(`{"client_secret":"secret-b"}`),
		Actor:   actor,
	})
	require.NoError(t, err, "changing the operator's own fields must stay allowed")
}

// Create trims the name instead of refusing a padded one.
//
// The name is the row's IMMUTABLE key -- it seals the client_secret's AAD,
// into the secret's AAD and into every URL that addresses the row -- so the
// question was never "trim or refuse" alone: it was whether an operator who
// typed " google " should get a row they then cannot find under that name.
// Trimming answers it the way any operator would expect, and " google " and
// "google" are not two names anyone means to tell apart.
//
// The stored name is asserted through a LOOKUP by the trimmed value, because
// that is the property that matters: a row created from padded input has to be
// reachable by the name an operator would type next.
func TestCreate_TrimsTheName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	actor := testActor()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    "  " + kinds.oidc + "  ",
		Enabled: lo.ToPtr(true),
		Config:  presetConfig("padded-client"),
		Secrets: json.RawMessage(`{"client_secret":"s"}`),
		Actor:   actor,
	})
	require.NoError(t, err, "a padded name is trimmed, not refused")

	got, err := svc.GetByKindName(ctx, kinds.login, kinds.oidc)
	require.NoError(t, err, "the row must be reachable by the trimmed name")
	require.Equal(t, kinds.oidc, got.Name, "the stored name carries no padding")
}

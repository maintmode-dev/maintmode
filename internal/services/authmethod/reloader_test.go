package authmethod

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/gateways/oidcdiscovery"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

type fakeRegistry struct {
	providers []entity.ConfiguredProvider
	err       error
	calls     int
	// askedFor is the category the reloader asked for.
	askedFor string
}

// The argument is RECORDED, not discarded. Dropping it in the signature is what
// let the reloader ask the registry for the wrong category without a single
// test noticing -- and asking for "notify" returns zero login rows, so
// installProviders would install nothing and sign-in would die with no error
// anywhere, because an empty result is not a failure.
func (f *fakeRegistry) ListLoginProviders(_ context.Context, category string) (
	[]entity.ConfiguredProvider, error,
) {
	f.calls++
	f.askedFor = category

	return f.providers, f.err
}

// captureInstaller is written by the reloader's goroutine and read by the test,
// so it is synchronized: the production installer is an atomic pointer swap,
// and a fixture that raced would fail the whole package under -race while
// telling us nothing about the code under test.
type captureInstaller struct {
	mu        sync.Mutex
	installed []providerInput
	calls     int
}

func (c *captureInstaller) installProviders(providers []providerInput) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.installed = providers
	c.calls++
}

func (c *captureInstaller) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.calls
}

func (c *captureInstaller) byID(id entity.AuthMethod) (providerInput, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range c.installed {
		if p.ID == id {
			return p, true
		}
	}

	return providerInput{}, false
}

func (c *captureInstaller) snapshot() []providerInput {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.installed)
}

// testResolver is a discovery resolver these tests never let anything reach.
//
// Building resolves nothing, so the real path works here: what a rebuild
// refuses is a row whose settings are the wrong shape, which is exactly the
// failure these tests exercise.
func testResolver() discoveryResolver {
	return oidcdiscovery.New()
}

func storedProvider(name string, enabled bool) entity.ConfiguredProvider {
	return entity.ConfiguredProvider{
		Name:     name,
		Enabled:  enabled,
		Settings: integrationkinds.OIDCSettings{DisplayName: name + " SSO", IssuerURL: "https://" + name + ".example"},
	}
}

// Criterion 1, at the level where it is decided: two stored providers both
// become usable in one rebuild.
func TestReload_TwoProvidersBothUsable(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{providers: []entity.ConfiguredProvider{
		storedProvider("google", true),
		storedProvider("keycloak", true),
	}}
	installer := &captureInstaller{}

	NewReloader(registry, installer, testResolver()).reload(context.Background())

	require.Len(t, installer.snapshot(), 2)
	for _, name := range []entity.AuthMethod{"google", "keycloak"} {
		got, ok := installer.byID(name)
		require.True(t, ok, "provider %q must be installed", name)
		require.NotNil(t, got.Method, "provider %q must be verifiable", name)
		require.NotNil(t, got.Gateway, "provider %q must be danceable", name)
		require.Equal(t, entity.LoginProviderHealthOK, got.Health)
	}
}

// Criterion 4: one provider that will not build disables ITS OWN row and
// nothing else -- the difference between a provider outage and a login outage.
//
// It used to be an unreachable ISSUER that triggered this, back when building
// resolved discovery. It no longer does: a rebuild reads stored rows and never
// leaves the process, so what can still fail here is the row itself -- settings
// that are not the shape this builder understands.
func TestReload_OneFailingProviderIsIsolated(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{providers: []entity.ConfiguredProvider{
		storedProvider("google", true),
		// A delivery row sitting in the login category: settings that are not
		// the shape the builder understands, which is the only way a build can
		// fail now that it reads the row and never leaves the process.
		{Name: "broken", Enabled: true, Settings: integrationkinds.SlackSettings{}},
	}}
	installer := &captureInstaller{}

	NewReloader(registry, installer, testResolver()).reload(context.Background())

	working, ok := installer.byID("google")
	require.True(t, ok)
	require.NotNil(t, working.Gateway, "a healthy provider must be unaffected by a broken neighbor")
	require.Equal(t, entity.LoginProviderHealthOK, working.Health)

	broken, ok := installer.byID("broken")
	require.True(t, ok, "a provider that did not build is still reported, so an operator can see why")
	require.Nil(t, broken.Gateway, "it must not be danceable")
	require.Equal(t, entity.LoginProviderHealthUnreadable, broken.Health)
}

// Disabling must stop the provider, not merely hide it: leaving the method
// behind would make "turn this off" advisory.
func TestReload_DisabledProviderIsNeitherVerifiableNorDanceable(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{providers: []entity.ConfiguredProvider{storedProvider("google", false)}}
	installer := &captureInstaller{}

	NewReloader(registry, installer, testResolver()).reload(context.Background())

	got, ok := installer.byID("google")
	require.True(t, ok)
	require.Nil(t, got.Method)
	require.Nil(t, got.Gateway)
	require.Equal(t, entity.LoginProviderHealthDisabled, got.Health)
}

// A row whose secret will not open is reported as such rather than silently
// skipped, and it must not take the rebuild down with it.
func TestReload_UnreadableProviderIsReported(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{providers: []entity.ConfiguredProvider{
		{Name: "broken", Enabled: true, Unreadable: true},
		storedProvider("google", true),
	}}
	installer := &captureInstaller{}

	NewReloader(registry, installer, testResolver()).reload(context.Background())

	broken, ok := installer.byID("broken")
	require.True(t, ok)
	require.Equal(t, entity.LoginProviderHealthUnreadable, broken.Health)
	require.Nil(t, broken.Gateway)

	working, ok := installer.byID("google")
	require.True(t, ok)
	require.Equal(t, entity.LoginProviderHealthOK, working.Health)
}

// A registry outage must not become a login outage: the previous snapshot stays
// live, which means installing nothing rather than installing an empty set.
func TestReload_RegistryFailureKeepsThePreviousSnapshot(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{err: errors.New("database is down")}
	installer := &captureInstaller{}

	r := NewReloader(registry, installer, testResolver())
	r.reload(context.Background())

	require.Zero(t, installer.callCount(),
		"a failed read must leave the live configuration alone, not replace it with nothing")

	// And the change that prompted it is still pending: the wake is re-armed,
	// so the loop comes straight back to the rebuild. Swallowing it would leave
	// an operator's save unapplied with nothing indicating why -- the
	// difference between "retried immediately" and "retried in thirty
	// seconds", and the comment on reload() promises the former.
	require.Len(t, r.wake, 1, "a failed rebuild must re-arm the wake")
}

// The registry is the ONLY source now.
//
// Providers used to also arrive from the config file and win a name collision
// with a stored row -- the break-glass path for an operator locked out of the
// admin UI. That half is gone with the config section, so a stored row is
// served rather than shadowed, and the bootstrap admin is the remaining way in.
func TestReload_StoredRowsAreTheOnlySource(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{providers: []entity.ConfiguredProvider{
		storedProvider("google", true),
		storedProvider("custom", true),
	}}
	installer := &captureInstaller{}

	NewReloader(registry, installer, testResolver()).reload(context.Background())

	require.Len(t, installer.snapshot(), 2, "every stored row must reach the snapshot")
}

// The change hook must not do the work: the rebuild resolves discovery, and
// doing that inline would hold an admin's HTTP response open for seconds.
func TestOnIntegrationChanged_DoesNotReloadInline(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{}
	r := NewReloader(registry, &captureInstaller{}, testResolver())

	r.OnIntegrationChanged(integrationkinds.CategoryLogin, "google")

	require.Zero(t, registry.calls, "the hook must only signal; the loop does the work")
	require.Len(t, r.wake, 1, "the hook must leave a rebuild pending for the loop")
}

// A delivery integration changing must not rebuild login.
func TestOnIntegrationChanged_IgnoresOtherKinds(t *testing.T) {
	t.Parallel()

	r := NewReloader(&fakeRegistry{}, &captureInstaller{}, testResolver())

	r.OnIntegrationChanged(integrationkinds.CategoryNotify, "slack")

	require.Empty(t, r.wake, "a delivery change must not schedule a login rebuild")
}

// The hook keys on the CATEGORY, so EVERY login provider reaches the snapshot
// -- not just the one that used to be the "oidc" kind. Reverting the comparison
// to a provider name passes for "google" and silently stops working for the
// rest, which is the shape this asserts against.
func TestOnIntegrationChanged_FiresForEveryLoginProvider(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"google", "custom"} {
		r := NewReloader(&fakeRegistry{}, &captureInstaller{}, testResolver())

		r.OnIntegrationChanged(integrationkinds.CategoryLogin, name)

		require.Lenf(t, r.wake, 1, "a change to %q must schedule a rebuild", name)
	}
}

// The hook has to actually drive a rebuild. TestOnIntegrationChanged_* proves
// it does NOT work inline; nothing proved it works at all, and a Run loop that
// drained the wake channel without reloading would have passed both.
func TestRun_ChangeHookTriggersAReload(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{providers: []entity.ConfiguredProvider{storedProvider("google", true)}}
	installer := &captureInstaller{}
	r := NewReloader(registry, installer, testResolver())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run loads once synchronously before the loop starts, so the hook's effect
	// is counted from THERE. Asserting callCount() > 0 would be satisfied by
	// that first load alone and would pass on a loop that drains the wake
	// channel without reloading -- which is the regression this exists to
	// catch.
	r.Run(ctx)
	afterStart := installer.callCount()
	require.Equal(t, 1, afterStart, "Run must build the snapshot before it returns")

	r.OnIntegrationChanged(integrationkinds.CategoryLogin, "google")

	// Waited for, not slept on: the loop is asynchronous by design, so the test
	// asserts the outcome arrives rather than guessing how long it takes.
	require.Eventually(t, func() bool { return installer.callCount() > afterStart },
		5*time.Second, 5*time.Millisecond,
		"a registry change must reach the snapshot without anything else prodding it")
}

// A provider whose label is blank still needs a button people can read, so the
// instance name stands in. Untested, the fallback reads as decoration -- and
// the page would show a nameless button nobody could identify.
func TestReload_BlankDisplayNameFallsBackToTheInstanceName(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{providers: []entity.ConfiguredProvider{{
		Name:     "keycloak",
		Enabled:  true,
		Settings: integrationkinds.OIDCSettings{IssuerURL: "https://idp.example"},
	}}}
	installer := &captureInstaller{}

	NewReloader(registry, installer, testResolver()).reload(context.Background())

	got, ok := installer.byID("keycloak")
	require.True(t, ok)
	require.Equal(t, "keycloak", got.DisplayName, "a button is never blank")
}

// The reloader must ask the registry for the LOGIN category.
//
// Nothing else pins this. The service-level test calls ListLoginProviders with
// the constant itself, so it proves the service, not what the reloader asks it
// for; the fake used to drop the argument in its signature, leaving the seam
// between them untested. Asking for "notify" returns zero login rows, and an
// empty result is not an error -- so the snapshot would be installed empty and
// sign-in would be gone with nothing logged.
func TestReload_AsksTheRegistryForTheLoginCategory(t *testing.T) {
	t.Parallel()

	registry := &fakeRegistry{}
	NewReloader(registry, &captureInstaller{}, testResolver()).reload(context.Background())

	require.Equal(t, integrationkinds.CategoryLogin, registry.askedFor,
		"asking for any other category returns no providers and empties the snapshot")
}

// The redirect scheme has to travel from the stored row into the snapshot
// input, and that ASSIGNMENT is what this pins.
//
// Both ends were already covered and the wire between them was not:
// TestRedirectsOverPlainHTTP proves the predicate, TestSnapshot_DanceCookieSecure
// proves the aggregation, and dropping the assignment between them left every
// test green. It is the cross-layer-assignment shape -- the compiler, a green
// suite and coverage all agree while the value never arrives.
//
// The failure points the unsafe way: the field stays false, so no provider ever
// counts as plain, so the cookie keeps Secure on a plain-http stand, where the
// browser withholds it and sign-in fails as a 302 that reads as success.
func TestReload_CarriesTheRedirectSchemeIntoTheSnapshot(t *testing.T) {
	t.Parallel()

	for name, redirect := range map[string]string{
		"plain http redirect": "http://localhost:9000/cb",
		"https redirect":      "https://app.example/cb",
	} {
		row := storedProvider("google", true)
		row.Settings = integrationkinds.OIDCSettings{
			DisplayName: "Google",
			IssuerURL:   "https://accounts.google.com",
			ClientID:    "c",
			RedirectURI: redirect,
		}

		installer := &captureInstaller{}
		NewReloader(&fakeRegistry{providers: []entity.ConfiguredProvider{row}},
			installer, testResolver()).reload(context.Background())

		current := installer.snapshot()
		require.Lenf(t, current, 1, "%s: the provider must reach the snapshot", name)
		require.Equalf(t, redirect, current[0].RedirectURI,
			"%s: the redirect must travel from the row into the snapshot input", name)
	}
}

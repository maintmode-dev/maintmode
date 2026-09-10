package authmethod_test

import (
	"context"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/authmethod"
)

// fakeProvider is a minimal AuthMethod stand-in. The registry only ever calls
// MethodID() on the providers it is given (to key the map), so Authenticate is
// here to satisfy the interface and records nothing.
type fakeProvider struct {
	id entity.AuthMethod
}

func (f *fakeProvider) MethodID() entity.AuthMethod {
	return f.id
}

func (f *fakeProvider) Authenticate(_ context.Context, _ string) (*entity.OAuthIDTokenClaims, error) {
	return &entity.OAuthIDTokenClaims{Subject: string(f.id)}, nil
}

func newConfig(env config.Environment, useStub bool) *config.AppConfig {
	return &config.AppConfig{
		Environment: env,
		OauthProviders: config.OauthProviders{
			UseStub: useStub,
		},
	}
}

func TestProvidersGet(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	t.Run("returns the registered google provider by name", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.AuthMethodGoogle}
		providers := authmethod.NewAuthMethods(
			newConfig(config.ProdEnvironment, false),
			[]authmethod.AuthMethod{google},
		)

		got, err := providers.Get(ctx, entity.AuthMethodGoogle)
		require.NoError(t, err)
		// Identity, not just the provider ID: Get must hand back the very instance
		// that was registered, otherwise a lookup could silently resolve to some
		// other provider that happens to report the same ID.
		require.Same(t, google, got)
		require.Equal(t, entity.AuthMethodGoogle, got.MethodID())
	})

	t.Run("returns the stub when useStub is true and env is dev", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.AuthMethodGoogle}
		providers := authmethod.NewAuthMethods(
			newConfig(config.DevEnvironment, true),
			[]authmethod.AuthMethod{google},
		)

		// The stub short-circuits every lookup in dev, including a request for a
		// provider that is registered under a different name.
		got, err := providers.Get(ctx, entity.AuthMethodGoogle)
		require.NoError(t, err)
		require.Equal(t, entity.AuthMethodStub, got.MethodID())
		require.NotSame(t, google, got)
	})

	t.Run("stub short-circuits even an unknown provider name in dev", func(t *testing.T) {
		t.Parallel()

		providers := authmethod.NewAuthMethods(
			newConfig(config.LocalEnvironment, true),
			[]authmethod.AuthMethod{&fakeProvider{id: entity.AuthMethodGoogle}},
		)

		// IsDev() is true for "local" too, so the stub gate opens there as well and
		// the unknown-provider branch is never reached.
		got, err := providers.Get(ctx, entity.AuthMethodGithub)
		require.NoError(t, err)
		require.Equal(t, entity.AuthMethodStub, got.MethodID())
	})

	t.Run("does not return the stub when useStub is true but env is not dev", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.AuthMethodGoogle}
		providers := authmethod.NewAuthMethods(
			newConfig(config.ProdEnvironment, true),
			[]authmethod.AuthMethod{google},
		)

		// This is the security-relevant half of the AND at provider.go:46 — a stray
		// use_stub=true in a prod config must not turn every login into a stub login.
		got, err := providers.Get(ctx, entity.AuthMethodGoogle)
		require.NoError(t, err)
		require.Same(t, google, got)
		require.Equal(t, entity.AuthMethodGoogle, got.MethodID())
	})

	t.Run("does not return the stub when env is dev but useStub is false", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.AuthMethodGoogle}
		providers := authmethod.NewAuthMethods(
			newConfig(config.DevEnvironment, false),
			[]authmethod.AuthMethod{google},
		)

		// The other half of the AND: dev alone must not enable the stub.
		got, err := providers.Get(ctx, entity.AuthMethodGoogle)
		require.NoError(t, err)
		require.Same(t, google, got)
	})

	t.Run("unknown provider is a wrapped ErrUnsupportedProvider", func(t *testing.T) {
		t.Parallel()

		providers := authmethod.NewAuthMethods(
			newConfig(config.ProdEnvironment, false),
			[]authmethod.AuthMethod{&fakeProvider{id: entity.AuthMethodGoogle}},
		)

		got, err := providers.Get(ctx, entity.AuthMethodGithub)
		require.Nil(t, got)
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
	})

	// ParseAuthMethod accepts "email" and "bootstrap" before anything
	// implements them. This pins the other half of that arrangement: the registry
	// must NOT resolve them, so a value that clears the parser still gets refused.
	// It is deliberately checked here rather than only through the invitation
	// service, because this assertion needs no config on disk and fails the moment
	// someone registers a method ahead of its implementation.
	t.Run("methods with no implementation are not registered", func(t *testing.T) {
		t.Parallel()

		// AuthMethodBootstrap left this list once it gained an implementation;
		// AuthMethodEmail is still vocabulary-only.
		for _, method := range []entity.AuthMethod{entity.AuthMethodEmail} {
			methods := authmethod.NewAuthMethods(
				newConfig(config.DevEnvironment, false),
				[]authmethod.AuthMethod{&fakeProvider{id: entity.AuthMethodGoogle}},
			)

			got, err := methods.Get(ctx, method)
			require.Nil(t, got, "method %q must not resolve", method)
			require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
		}
	})

	t.Run("unknown provider error names the provider that was requested", func(t *testing.T) {
		t.Parallel()

		providers := authmethod.NewAuthMethods(
			newConfig(config.ProdEnvironment, false),
			[]authmethod.AuthMethod{&fakeProvider{id: entity.AuthMethodGoogle}},
		)

		_, err := providers.Get(ctx, entity.AuthMethodGithub)
		require.Error(t, err)

		// This string is not internal-only: httperrors.mapper puts err.Error()
		// straight into the 400 body, so it is what an API client reads. It must
		// name the provider — formatting the nil interface from the failed lookup
		// instead of providerName renders "%!s(<nil>)", which tells a caller
		// nothing about what they got wrong.
		require.EqualError(t, err, "unsupported provider: github")
	})

	t.Run("the stub is not registered outside dev", func(t *testing.T) {
		t.Parallel()

		providers := authmethod.NewAuthMethods(
			newConfig(config.ProdEnvironment, false),
			[]authmethod.AuthMethod{&fakeProvider{id: entity.AuthMethodGoogle}},
		)

		// The stub accepts any token and mints an identity, so outside dev it must
		// not exist rather than merely be unreachable through the useStub gate.
		// Asking for it by name is indistinguishable from asking for any other
		// unknown provider.
		got, err := providers.Get(ctx, entity.AuthMethodStub)
		require.Nil(t, got)
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
	})

	t.Run("the stub is unreachable in prod even when useStub is set", func(t *testing.T) {
		t.Parallel()

		providers := authmethod.NewAuthMethods(
			newConfig(config.ProdEnvironment, true),
			[]authmethod.AuthMethod{&fakeProvider{id: entity.AuthMethodGoogle}},
		)

		// A stray use_stub=true in a prod config must not resurrect the stub by
		// name either — registration and the useStub gate are derived from the
		// same isDev, so neither half can open on its own.
		got, err := providers.Get(ctx, entity.AuthMethodStub)
		require.Nil(t, got)
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
	})

	t.Run("prod with useStub still resolves a real provider", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.AuthMethodGoogle}
		providers := authmethod.NewAuthMethods(
			newConfig(config.ProdEnvironment, true),
			[]authmethod.AuthMethod{google},
		)

		// Guards the failure mode the shared isDev exists to prevent: a useStub
		// that stayed true while the stub went unregistered would make Get
		// short-circuit to a missing entry and fail for EVERY provider — a silent
		// total login outage rather than a security hole.
		got, err := providers.Get(ctx, entity.AuthMethodGoogle)
		require.NoError(t, err)
		require.Same(t, google, got)
	})

	t.Run("a registered provider overrides nothing else", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.AuthMethodGoogle}
		github := &fakeProvider{id: entity.AuthMethodGithub}
		providers := authmethod.NewAuthMethods(
			newConfig(config.ProdEnvironment, false),
			[]authmethod.AuthMethod{google, github},
		)

		gotGoogle, err := providers.Get(ctx, entity.AuthMethodGoogle)
		require.NoError(t, err)
		require.Same(t, google, gotGoogle)

		gotGithub, err := providers.Get(ctx, entity.AuthMethodGithub)
		require.NoError(t, err)
		require.Same(t, github, gotGithub)
	})
}

// The stub short-circuit must not swallow the break-glass method.
//
// Get rewrites the requested method to AuthMethodStub whenever useStub is on,
// and useStub is `isDev && cfg.OauthProviders.UseStub` — both halves matter, so
// this must be exercised with an IsDev environment AND the flag, or the
// short-circuit is not live and the test would pass with the exclusion removed.
//
// Without the exclusion a bootstrap login on the dev or test stand (both ship
// use_stub: true) authenticates through the stub, which accepts any credential
// and reports Subject "stub" — creating a DIFFERENT user while appearing to
// work. That is worse than an error: it silently breaks "a repeat login
// resolves the same user".
func TestProvidersGet_BootstrapBypassesTheStub(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	bootstrap := &fakeProvider{id: entity.AuthMethodBootstrap}
	stub := &fakeProvider{id: entity.AuthMethodStub}
	methods := authmethod.NewAuthMethods(
		newConfig(config.DevEnvironment, true),
		[]authmethod.AuthMethod{bootstrap, stub, &fakeProvider{id: entity.AuthMethodGoogle}},
	)

	got, err := methods.Get(ctx, entity.AuthMethodBootstrap)
	require.NoError(t, err)
	require.Same(t, bootstrap, got, "bootstrap must never resolve to the stub")

	// The short-circuit itself is untouched for everyone else: google still
	// resolves to the stub under the same config.
	got, err = methods.Get(ctx, entity.AuthMethodGoogle)
	require.NoError(t, err)
	require.Equal(t, entity.AuthMethodStub, got.MethodID(),
		"the stub substitution must still apply to other methods")
}

// TestMethodsParse covers the vocabulary a client may name in a request.
//
// It used to be a closed switch in entity; it is the registry now, so an
// instance added to configuration is accepted without a code change. What
// survives from the old function is the pair of refusals, and both are here
// because both are security properties rather than tidiness.
func TestMethodsParse(t *testing.T) {
	t.Parallel()

	methods := authmethod.NewAuthMethods(
		newConfig(config.ProdEnvironment, false),
		[]authmethod.AuthMethod{
			&fakeProvider{id: entity.AuthMethodGoogle},
			// A name that no compiled-in list ever knew about: this is the whole
			// point of configuring providers instead of compiling them in.
			&fakeProvider{id: entity.AuthMethod("acme")},
			&fakeProvider{id: entity.AuthMethodBootstrap},
		},
	)

	t.Run("accepts a registered provider", func(t *testing.T) {
		t.Parallel()

		got, ok := methods.Parse("google")
		require.True(t, ok)
		require.Equal(t, entity.AuthMethodGoogle, got)
	})

	t.Run("accepts a provider that exists only in configuration", func(t *testing.T) {
		t.Parallel()

		got, ok := methods.Parse("acme")
		require.True(t, ok)
		require.Equal(t, entity.AuthMethod("acme"), got)
	})

	t.Run("refuses a name nothing registered", func(t *testing.T) {
		t.Parallel()

		_, ok := methods.Parse("okta")
		require.False(t, ok)
	})

	// Refused even though it IS registered: break-glass resolves an identity by
	// configured email and grants admin past the seats cap, which is safe only
	// on the endpoint gating it behind the secret. A client naming it elsewhere
	// would carry those privileges onto a flow that never intended them.
	t.Run("refuses bootstrap even though it is registered", func(t *testing.T) {
		t.Parallel()

		_, ok := methods.Parse("bootstrap")
		require.False(t, ok)
	})

	// The stub accepts any credential. On a dev stand it IS registered, so the
	// refusal cannot lean on absence.
	t.Run("refuses the stub where it is registered", func(t *testing.T) {
		t.Parallel()

		dev := authmethod.NewAuthMethods(
			newConfig(config.DevEnvironment, false),
			[]authmethod.AuthMethod{&fakeProvider{id: entity.AuthMethodGoogle}},
		)

		_, ok := dev.Parse("stub")
		require.False(t, ok)
	})

	// Exact matching: folding would let "STUB" smuggle the stub back past the
	// gate above.
	t.Run("does not case-fold", func(t *testing.T) {
		t.Parallel()

		_, ok := methods.Parse("Google")
		require.False(t, ok)
	})
}

// TestMethodsDanceProvider covers the narrower vocabulary of the {provider}
// path segment on the backend dance.
//
// Narrower because a dance needs the confidential-client credentials: an
// instance configured for the BFF path alone has no gateway to send the browser
// through. The check runs before /start mints anything, and the segment shares
// a path space with the static /login/oauth/code/exchange route.
func TestMethodsDanceProvider(t *testing.T) {
	t.Parallel()

	methods := authmethod.NewAuthMethods(
		newConfig(config.ProdEnvironment, false),
		[]authmethod.AuthMethod{
			&fakeProvider{id: entity.AuthMethodGoogle},
			&fakeProvider{id: entity.AuthMethod("acme")},
		},
	).WithDanceProviders([]string{"acme"})

	t.Run("accepts a provider with dance credentials", func(t *testing.T) {
		t.Parallel()

		got, ok := methods.DanceProvider("acme")
		require.True(t, ok)
		require.Equal(t, entity.AuthMethod("acme"), got)
	})

	t.Run("refuses a registered provider that cannot dance", func(t *testing.T) {
		t.Parallel()

		_, ok := methods.DanceProvider("google")
		require.False(t, ok)
	})

	t.Run("refuses a name nothing registered", func(t *testing.T) {
		t.Parallel()

		_, ok := methods.DanceProvider("okta")
		require.False(t, ok)
	})
}

// TestStubSubstitutionYieldsVerifiedClaims covers the regression the
// email_verified guard would otherwise cause on every use_stub stand.
//
// Get substitutes the stub for whatever method was asked for, so on dev and
// test stands an invitation accept -- which compares the provider's email
// against the invited one -- runs through the stub rather than through a real
// OIDC provider. The stub has no upstream to have checked anything, so if its
// claims left EmailVerified at the zero value, every accept on those stands
// would collapse into an opaque email-mismatch refusal.
//
// Bootstrap is exempt from the substitution and reports the flag by its own
// path, for the same reason: its address comes from configuration.
func TestStubSubstitutionYieldsVerifiedClaims(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	methods := authmethod.NewAuthMethods(
		newConfig(config.DevEnvironment, true),
		[]authmethod.AuthMethod{&fakeProvider{id: entity.AuthMethodGoogle}},
	)

	method, err := methods.Get(ctx, entity.AuthMethodGoogle)
	require.NoError(t, err)
	require.Equal(t, entity.AuthMethodStub, method.MethodID(), "use_stub must substitute the stub")

	claims, err := method.Authenticate(ctx, "invited@example.com")
	require.NoError(t, err)
	require.True(t, claims.EmailVerified,
		"the stub has no upstream, so its claims must not read as an issuer refusing to vouch")
}

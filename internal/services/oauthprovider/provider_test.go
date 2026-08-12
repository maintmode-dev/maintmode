package oauthprovider_test

import (
	"context"
	"testing"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/oauthprovider"
)

// fakeProvider is a minimal OAuthProvider stand-in. The registry only ever calls
// ProviderID() on the providers it is given (to key the map), so VerifyToken is
// here to satisfy the interface and records nothing.
type fakeProvider struct {
	id entity.OAuthProvider
}

func (f *fakeProvider) ProviderID() entity.OAuthProvider {
	return f.id
}

func (f *fakeProvider) VerifyToken(_ context.Context, _ string) (*entity.OAuthIDTokenClaims, error) {
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

		google := &fakeProvider{id: entity.OAuthProviderGoogle}
		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.ProdEnvironment, false),
			[]oauthprovider.OAuthProvider{google},
		)

		got, err := providers.Get(ctx, entity.OAuthProviderGoogle)
		require.NoError(t, err)
		// Identity, not just the provider ID: Get must hand back the very instance
		// that was registered, otherwise a lookup could silently resolve to some
		// other provider that happens to report the same ID.
		require.Same(t, google, got)
		require.Equal(t, entity.OAuthProviderGoogle, got.ProviderID())
	})

	t.Run("returns the stub when useStub is true and env is dev", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.OAuthProviderGoogle}
		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.DevEnvironment, true),
			[]oauthprovider.OAuthProvider{google},
		)

		// The stub short-circuits every lookup in dev, including a request for a
		// provider that is registered under a different name.
		got, err := providers.Get(ctx, entity.OAuthProviderGoogle)
		require.NoError(t, err)
		require.Equal(t, entity.OAuthProviderStub, got.ProviderID())
		require.NotSame(t, google, got)
	})

	t.Run("stub short-circuits even an unknown provider name in dev", func(t *testing.T) {
		t.Parallel()

		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.LocalEnvironment, true),
			[]oauthprovider.OAuthProvider{&fakeProvider{id: entity.OAuthProviderGoogle}},
		)

		// IsDev() is true for "local" too, so the stub gate opens there as well and
		// the unknown-provider branch is never reached.
		got, err := providers.Get(ctx, entity.OAuthProviderGithub)
		require.NoError(t, err)
		require.Equal(t, entity.OAuthProviderStub, got.ProviderID())
	})

	t.Run("does not return the stub when useStub is true but env is not dev", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.OAuthProviderGoogle}
		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.ProdEnvironment, true),
			[]oauthprovider.OAuthProvider{google},
		)

		// This is the security-relevant half of the AND at provider.go:46 — a stray
		// use_stub=true in a prod config must not turn every login into a stub login.
		got, err := providers.Get(ctx, entity.OAuthProviderGoogle)
		require.NoError(t, err)
		require.Same(t, google, got)
		require.Equal(t, entity.OAuthProviderGoogle, got.ProviderID())
	})

	t.Run("does not return the stub when env is dev but useStub is false", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.OAuthProviderGoogle}
		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.DevEnvironment, false),
			[]oauthprovider.OAuthProvider{google},
		)

		// The other half of the AND: dev alone must not enable the stub.
		got, err := providers.Get(ctx, entity.OAuthProviderGoogle)
		require.NoError(t, err)
		require.Same(t, google, got)
	})

	t.Run("unknown provider is a wrapped ErrUnsupportedProvider", func(t *testing.T) {
		t.Parallel()

		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.ProdEnvironment, false),
			[]oauthprovider.OAuthProvider{&fakeProvider{id: entity.OAuthProviderGoogle}},
		)

		got, err := providers.Get(ctx, entity.OAuthProviderGithub)
		require.Nil(t, got)
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
	})

	t.Run("unknown provider error names the provider that was requested", func(t *testing.T) {
		t.Parallel()

		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.ProdEnvironment, false),
			[]oauthprovider.OAuthProvider{&fakeProvider{id: entity.OAuthProviderGoogle}},
		)

		_, err := providers.Get(ctx, entity.OAuthProviderGithub)
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

		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.ProdEnvironment, false),
			[]oauthprovider.OAuthProvider{&fakeProvider{id: entity.OAuthProviderGoogle}},
		)

		// The stub accepts any token and mints an identity, so outside dev it must
		// not exist rather than merely be unreachable through the useStub gate.
		// Asking for it by name is indistinguishable from asking for any other
		// unknown provider (RUK-249).
		got, err := providers.Get(ctx, entity.OAuthProviderStub)
		require.Nil(t, got)
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
	})

	t.Run("the stub is unreachable in prod even when useStub is set", func(t *testing.T) {
		t.Parallel()

		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.ProdEnvironment, true),
			[]oauthprovider.OAuthProvider{&fakeProvider{id: entity.OAuthProviderGoogle}},
		)

		// A stray use_stub=true in a prod config must not resurrect the stub by
		// name either — registration and the useStub gate are derived from the
		// same isDev, so neither half can open on its own.
		got, err := providers.Get(ctx, entity.OAuthProviderStub)
		require.Nil(t, got)
		require.ErrorIs(t, err, apperr.ErrUnsupportedProvider)
	})

	t.Run("prod with useStub still resolves a real provider", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.OAuthProviderGoogle}
		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.ProdEnvironment, true),
			[]oauthprovider.OAuthProvider{google},
		)

		// Guards the failure mode the shared isDev exists to prevent: a useStub
		// that stayed true while the stub went unregistered would make Get
		// short-circuit to a missing entry and fail for EVERY provider — a silent
		// total login outage rather than a security hole.
		got, err := providers.Get(ctx, entity.OAuthProviderGoogle)
		require.NoError(t, err)
		require.Same(t, google, got)
	})

	t.Run("a registered provider overrides nothing else", func(t *testing.T) {
		t.Parallel()

		google := &fakeProvider{id: entity.OAuthProviderGoogle}
		github := &fakeProvider{id: entity.OAuthProviderGithub}
		providers := oauthprovider.NewOAuthProviders(
			newConfig(config.ProdEnvironment, false),
			[]oauthprovider.OAuthProvider{google, github},
		)

		gotGoogle, err := providers.Get(ctx, entity.OAuthProviderGoogle)
		require.NoError(t, err)
		require.Same(t, google, gotGoogle)

		gotGithub, err := providers.Get(ctx, entity.OAuthProviderGithub)
		require.NoError(t, err)
		require.Same(t, github, gotGithub)
	})
}

package authmethod

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

// Build carries five fields from the stored settings into two different
// objects. Each is a cross-layer assignment the compiler cannot check: swap two
// of them and everything still builds, while sign-in fails in a way that points
// nowhere near this function.
func TestReloader_BuildProvider(t *testing.T) {
	t.Parallel()

	t.Run("refuses settings of another kind", func(t *testing.T) {
		t.Parallel()

		got := newTestReloader().buildOne(context.Background(), "keycloak",
			entity.ConfiguredProvider{
				Name:     "keycloak",
				Enabled:  true,
				Settings: integrationkinds.SlackSettings{},
			})

		require.Equal(t, entity.LoginProviderHealthUnreadable, got.Health,
			"a type mismatch disables that provider, not the rebuild")
		require.Nil(t, got.Method, "nothing usable may be built from the wrong shape")
		require.Nil(t, got.Gateway)
	})

	// An unreachable issuer builds fine, and that is the point: a rebuild reads
	// the stored row and nothing else, so one IdP being down cannot stall or
	// fail it. The verifier resolves on first use instead -- the cost moves to
	// the first sign-in through that provider, and to nobody else.
	//
	// 127.0.0.1:1 is chosen because it cannot answer: if anything here reached
	// for discovery, this test would fail rather than pass slowly.
	t.Run("builds without reaching the issuer", func(t *testing.T) {
		t.Parallel()

		built := newTestReloader().buildOne(context.Background(), "unreachable",
			entity.ConfiguredProvider{
				Name:    "unreachable",
				Enabled: true,
				Settings: integrationkinds.OIDCSettings{
					IssuerURL: "https://127.0.0.1:1/nowhere",
					ClientID:  "client",
				},
			})

		require.Equal(t, entity.LoginProviderHealthOK, built.Health,
			"building must not depend on the IdP answering")
		require.NotNil(t, built.Method)
		require.NotNil(t, built.Gateway)
		require.Equal(t, "unreachable", built.DisplayName,
			"a provider with no label falls back to its instance name")
	})
}

// newTestReloader builds one with nothing wired but the resolver, which is all
// buildOne touches.
func newTestReloader() *Reloader {
	return NewReloader(nil, nil, testResolver())
}

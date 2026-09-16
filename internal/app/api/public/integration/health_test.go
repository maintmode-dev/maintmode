package integrationapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

type fixedHealth struct {
	value    string
	askedFor string
}

func (f *fixedHealth) ProviderHealth(name string) string {
	f.askedFor = name

	return f.value
}

func TestHealthOf(t *testing.T) {
	t.Parallel()

	// Keyed on the CATEGORY. Every login row carries health, not just the one
	// that used to be the "oidc" kind: comparing against a provider name here
	// would silently empty the field for every other provider.
	t.Run("reports the live health of a login provider", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"google", "custom"} {
			health := &fixedHealth{value: "unresolved"}
			impl := (&Implementation{}).WithLoginHealth(health)

			got := impl.healthOf(&entity.MaskedIntegration{
				Kind: integrationkinds.CategoryLogin, Name: name,
			})

			require.Equal(t, "unresolved", got, "health of %q", name)
			require.Equal(t, name, health.askedFor,
				"health is per provider: asking by category would answer for the wrong one")
		}
	})

	// A delivery integration has no IdP behind it to be unreachable, and its
	// reachability is already reported per channel elsewhere.
	t.Run("is absent for a delivery kind", func(t *testing.T) {
		t.Parallel()

		impl := (&Implementation{}).WithLoginHealth(&fixedHealth{value: "ok"})

		require.Empty(t, impl.healthOf(&entity.MaskedIntegration{
			Kind: integrationkinds.CategoryNotify, Name: "slack",
		}))
	})

	// An instance with no login providers wired has nothing to report, and must
	// answer rather than panic.
	t.Run("is absent when nothing is wired", func(t *testing.T) {
		t.Parallel()

		require.Empty(t, (&Implementation{}).healthOf(&entity.MaskedIntegration{
			Kind: integrationkinds.CategoryLogin, Name: "google",
		}))
	})
}

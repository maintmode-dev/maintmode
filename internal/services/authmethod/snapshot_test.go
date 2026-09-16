package authmethod

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
)

type fakeMethod struct{ id entity.AuthMethod }

func (f fakeMethod) MethodID() entity.AuthMethod { return f.id }

func (fakeMethod) Authenticate(context.Context, string) (*entity.OAuthIDTokenClaims, error) {
	return &entity.OAuthIDTokenClaims{}, nil
}

type fakeGateway struct{ url string }

func (f fakeGateway) AuthCodeURL(context.Context, string, string) (string, error) {
	return f.url, nil
}
func (fakeGateway) Exchange(context.Context, string, string) (string, error) { return "", nil }

func provider(id string, gw Gateway, health entity.LoginProviderHealth) providerInput {
	return providerInput{
		ID:          entity.AuthMethod(id),
		DisplayName: id,
		Method:      fakeMethod{id: entity.AuthMethod(id)},
		Gateway:     gw,
		Health:      health,
	}
}

func newMethods(t *testing.T, builtins map[entity.AuthMethod]AuthMethod) *Methods {
	t.Helper()

	m := &Methods{builtins: builtins}
	m.replace(newSnapshot(builtins, nil))

	return m
}

func TestSnapshot_InstallProviders(t *testing.T) {
	t.Parallel()

	t.Run("a resolved provider is listed, verifiable and danceable", func(t *testing.T) {
		t.Parallel()

		m := newMethods(t, nil)
		m.installProviders([]providerInput{provider("keycloak", fakeGateway{url: "https://idp/auth"}, entity.LoginProviderHealthOK)})

		_, err := m.Get(context.Background(), "keycloak")
		require.NoError(t, err)

		method, danceable := m.DanceProvider("keycloak")
		require.True(t, danceable)
		require.Equal(t, entity.AuthMethod("keycloak"), method)

		_, ok := m.DanceGateway("keycloak")
		require.True(t, ok)

		require.Equal(t, []entity.LoginMethodView{{ID: "keycloak", DisplayName: "keycloak"}}, m.snapshot().listing)
		require.Equal(t, entity.LoginProviderHealthOK, m.snapshot().healthOf("keycloak"))
	})

	// The failure this design exists to prevent: a button whose gateway is
	// missing. An unresolved provider must be visible and refuse, not vanish
	// (a brief IdP outage is not a deleted provider) and not half-work.
	t.Run("an unresolved provider is listed but not danceable", func(t *testing.T) {
		t.Parallel()

		m := newMethods(t, nil)
		m.installProviders([]providerInput{provider("keycloak", nil, entity.LoginProviderHealthUnresolved)})

		require.Len(t, m.snapshot().listing, 1, "a provider whose IdP is down must still be listed")

		_, danceable := m.DanceProvider("keycloak")
		require.False(t, danceable, "without a gateway it must refuse rather than mint state it cannot redeem")
		require.Equal(t, entity.LoginProviderHealthUnresolved, m.snapshot().healthOf("keycloak"))
	})

	t.Run("a disabled provider is not listed", func(t *testing.T) {
		t.Parallel()

		m := newMethods(t, nil)
		m.installProviders([]providerInput{provider("keycloak", nil, entity.LoginProviderHealthDisabled)})

		require.Empty(t, m.snapshot().listing)
		require.Equal(t, entity.LoginProviderHealthDisabled, m.snapshot().healthOf("keycloak"))
	})

	// The listing is a contract: two callers must get identical bytes, and map
	// iteration is randomized, so an unsorted rebuild would reshuffle the
	// sign-in buttons between requests and between replicas.
	t.Run("the listing is sorted", func(t *testing.T) {
		t.Parallel()

		m := newMethods(t, nil)
		m.installProviders([]providerInput{
			provider("zitadel", fakeGateway{}, entity.LoginProviderHealthOK),
			provider("auth0", fakeGateway{}, entity.LoginProviderHealthOK),
			provider("keycloak", fakeGateway{}, entity.LoginProviderHealthOK),
		})

		listing := m.snapshot().listing
		ids := make([]entity.AuthMethod, 0, len(listing))
		for _, v := range listing {
			ids = append(ids, v.ID)
		}
		require.Equal(t, []entity.AuthMethod{"auth0", "keycloak", "zitadel"}, ids)
	})

	// Losing the stub on a reload would break every sign-in on the dev and test
	// stands, where Get substitutes it for any method.
	t.Run("built-in methods survive a reload", func(t *testing.T) {
		t.Parallel()

		m := newMethods(t, map[entity.AuthMethod]AuthMethod{
			entity.AuthMethodStub:      fakeMethod{id: entity.AuthMethodStub},
			entity.AuthMethodBootstrap: fakeMethod{id: entity.AuthMethodBootstrap},
		})
		m.installProviders([]providerInput{provider("keycloak", fakeGateway{}, entity.LoginProviderHealthOK)})

		for _, id := range []entity.AuthMethod{entity.AuthMethodStub, entity.AuthMethodBootstrap} {
			_, err := m.Get(context.Background(), id)
			require.NoError(t, err, "built-in %q must survive a provider reload", id)
		}
	})

	// A provider removed from the registry must stop working, or disabling one
	// would be advisory.
	t.Run("a removed provider disappears", func(t *testing.T) {
		t.Parallel()

		m := newMethods(t, nil)
		m.installProviders([]providerInput{provider("keycloak", fakeGateway{}, entity.LoginProviderHealthOK)})
		m.installProviders(nil)

		_, err := m.Get(context.Background(), "keycloak")
		require.Error(t, err)
		require.Empty(t, m.snapshot().listing)
	})

	// A row that exists in the database but never made it into a snapshot is in
	// exactly the state "listed, and /start refuses" already describes.
	t.Run("an unknown provider reads as unresolved", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, entity.LoginProviderHealthUnresolved, newMethods(t, nil).snapshot().healthOf("never-seen"))
	})
}

// The reload happens under live traffic, so the readers must never observe a
// half-built configuration. The assertion is not "no race detected" but the
// stronger one the single pointer buys: whenever a provider is danceable, its
// gateway is there too.
func TestSnapshot_ConcurrentReloadNeverTears(t *testing.T) {
	t.Parallel()

	m := newMethods(t, nil)
	m.installProviders([]providerInput{provider("keycloak", fakeGateway{}, entity.LoginProviderHealthOK)})

	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		for i := 0; i < iterations; i++ {
			// Alternate between two shapes so a reader has something to catch
			// mid-swap if the halves could ever be written independently.
			if i%2 == 0 {
				m.installProviders([]providerInput{provider("keycloak", fakeGateway{}, entity.LoginProviderHealthOK)})
			} else {
				m.installProviders([]providerInput{
					provider("keycloak", fakeGateway{}, entity.LoginProviderHealthOK),
					provider("auth0", fakeGateway{}, entity.LoginProviderHealthOK),
				})
			}
		}
	}()

	go func() {
		defer wg.Done()

		for i := 0; i < iterations; i++ {
			current := m.snapshot()
			// "Danceable but no gateway" is no longer representable -- the
			// gateway map IS the danceable set -- so what is left to tear is
			// the pair that remains two maps: a provider that can complete a
			// dance must also be able to verify what comes back.
			for id := range current.gateways {
				_, verifiable := current.methods[id]
				require.True(t, verifiable,
					"provider %q has a gateway but cannot verify: the snapshot tore", id)
			}
			for _, view := range current.listing {
				_, verifiable := current.methods[view.ID]
				require.True(t, verifiable,
					"provider %q is listed but not verifiable: the snapshot tore", view.ID)
			}
		}
	}()

	wg.Wait()
}

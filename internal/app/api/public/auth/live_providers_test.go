package auth

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/services/authmethod"
)

// A provider installed at runtime has to become visible to ALL THREE readers,
// not just to the snapshot.
//
// Asserting the snapshot's contents would test the producer and miss the
// failure that actually matters: a reader still bound to its old source. The
// first draft of this work had exactly that bug -- the button list read a
// config copy pinned at construction, so a new provider would have been
// verifiable and danceable while remaining invisible on the sign-in page, with
// nothing failing to compile.
func TestLiveProviders_AllThreeReadersSeeANewProvider(t *testing.T) {
	t.Parallel()

	methods := authmethod.NewAuthMethods(cfg, nil)
	impl := initImpl(t).WithAuthMethods(methods)

	// Before: nothing configured.
	require.NotContains(t, listedIDs(t, impl), "keycloak")

	_, danceableBefore := methods.DanceProvider("keycloak")
	require.False(t, danceableBefore)

	// The reload an operator's save would trigger.
	installProviders(t, methods, testProvider{
		ID:          "keycloak",
		DisplayName: "Corporate SSO",
	})

	// Reader 1: the sign-in button list.
	require.Contains(t, listedIDs(t, impl), "keycloak",
		"the button must appear without a restart")

	// Reader 2: verification. Asserted through Parse rather than Get, because
	// this package's test config sets OauthProviders.UseStub, and Get
	// substitutes the stub for ANY method there -- it would answer "yes" for a
	// provider that does not exist, which is the use_stub trap this project has
	// been bitten by before. Parse consults the registered set directly.
	_, registered := methods.Parse("keycloak")
	require.True(t, registered, "the provider must be verifiable without a restart")

	// Reader 3: the dance, gateway included.
	method, danceable := methods.DanceProvider("keycloak")
	require.True(t, danceable, "the provider must be danceable without a restart")
	require.Equal(t, entity.AuthMethod("keycloak"), method)

	_, hasGateway := methods.DanceGateway("keycloak")
	require.True(t, hasGateway, "a danceable provider without a gateway is the failure this design prevents")
}

// Disabling one must take effect on every reader too, or turning a provider off
// would be advisory.
func TestLiveProviders_RemovalIsVisibleEverywhere(t *testing.T) {
	t.Parallel()

	methods := authmethod.NewAuthMethods(cfg, nil)
	impl := initImpl(t).WithAuthMethods(methods)

	installProviders(t, methods, testProvider{ID: "keycloak"})
	require.Contains(t, listedIDs(t, impl), "keycloak")

	installProviders(t, methods)

	require.NotContains(t, listedIDs(t, impl), "keycloak")

	// Parse, not Get: the stub substitution in this package's config would
	// answer for a provider that no longer exists.
	_, registered := methods.Parse("keycloak")
	require.False(t, registered, "a removed provider must stop being registered")

	_, danceable := methods.DanceProvider("keycloak")
	require.False(t, danceable)
}

// listedIDs reads the ids the sign-in endpoint renders.
func listedIDs(t *testing.T, impl *Implementation) []string {
	t.Helper()

	resp := doListAuthMethods(t, impl)
	require.Equal(t, http.StatusOK, resp.status)

	var body struct {
		Methods []struct {
			ID string `json:"id"`
		} `json:"methods"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp.body), &body))

	ids := make([]string, 0, len(body.Methods))
	for _, m := range body.Methods {
		ids = append(ids, m.ID)
	}

	return ids
}

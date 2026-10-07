package oidc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/ruko1202/xhttp/dialguard"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/gateways/oidc"
	"github.com/ruko1202/maintmode/internal/gateways/oidcdiscovery"
	mock_oidc "github.com/ruko1202/maintmode/internal/pkg/generated/mocks/gateways/oidc"
)

// The token endpoint comes from the discovery document, so whoever serves that
// document chooses where this process posts the client secret. An endpoint on
// an internal address must be refused before the connection is made.
func TestExchangeRefusesAnInternalTokenEndpoint(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","token_type":"Bearer","id_token":"x.y.z"}`))
	}))
	t.Cleanup(srv.Close)

	resolver := mock_oidc.NewMockresolver(gomock.NewController(t))
	resolver.EXPECT().
		Resolve(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, issuer string) (oidcdiscovery.Provider, error) {
			cfg := gooidc.ProviderConfig{IssuerURL: issuer, TokenURL: srv.URL + "/token"}

			return oidcdiscovery.Provider{OIDC: cfg.NewProvider(ctx), Issuer: issuer}, nil
		}).
		AnyTimes()

	gw := oidc.NewClient(config.OIDCProvider{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		IssuerURL:    "https://idp.example",
		RedirectURI:  "https://app.example/callback",
	}, resolver)

	idToken, err := gw.Exchange(t.Context(), "code", "verifier")
	require.Empty(t, idToken)
	require.ErrorIs(t, err, apperr.ErrOAuthExchangeFailed)
	require.ErrorIs(t, err, dialguard.ErrBlockedAddress)
	require.Zero(t, hits.Load(), "the secret must never reach an internal token endpoint")
}

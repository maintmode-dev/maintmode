package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ruko1202/xlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
)

// TestInitAuthMethods_RegistersBootstrapEverywhere drives the REAL wiring
// function rather than authmethod.NewAuthMethods.
//
// The distinction matters: NewAuthMethods registers nothing on its own, it only
// keys whatever slice the caller hands it. A registry-level test proves the map
// works and says nothing about whether this binary actually wires bootstrap in.
// Only this call site can fail when the method is dropped from the slice.
//
// Production is the environment that matters here. Gating registration on the
// environment — or on whether a password is configured — would recreate the
// loop break-glass exists to break: a clean prod instance with nothing
// configured would have no way in at all.
func TestInitAuthMethods_RegistersBootstrapEverywhere(t *testing.T) {
	t.Parallel()
	ctx := xlog.ContextWithLogger(context.Background(), xlog.NewZapAdapter(zaptest.NewLogger(t)))

	// A local JWKS endpoint keeps the Google provider's construction off the
	// network. Its first fetch is non-fatal by design (NoErrorReturnFirstHTTPReq),
	// so the content does not matter — only that nothing reaches out to Google.
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(jwks.Close)

	newCfg := func(env config.Environment, useStub bool) *config.AppConfig {
		return &config.AppConfig{
			Environment: env,
			OauthProviders: config.OauthProviders{
				UseStub: useStub,
				Google: config.GoogleOauthProvider{
					ClientID: "test-client-id",
					JWTVerify: config.JWTVerifierConfig{
						JWKSURL:                   jwks.URL,
						JWTIssuers:                []string{"https://accounts.google.com"},
						JWKSRefreshInterval:       time.Hour,
						JWKSHTTPTimeout:           time.Second,
						JWTLeeway:                 time.Second,
						JWKSUnknownKIDRefreshRate: time.Minute,
						JWKSUnknownKIDWaitMax:     time.Second,
					},
				},
			},
			Bootstrap: config.BootstrapConfig{Email: "admin@example.com"},
		}
	}

	tests := []struct {
		name    string
		env     config.Environment
		useStub bool
	}{
		{name: "prod", env: config.ProdEnvironment},
		{name: "dev", env: config.DevEnvironment},
		// The dev and test stands ship use_stub: true. Bootstrap must resolve to
		// its own implementation there too — the stub would accept any password
		// and report a different subject, so it would resolve a different user.
		{name: "dev with the stub enabled", env: config.DevEnvironment, useStub: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			methods, err := initAuthMethods(ctx, newCfg(tc.env, tc.useStub))
			require.NoError(t, err)

			got, err := methods.Get(ctx, entity.AuthMethodBootstrap)
			require.NoError(t, err, "bootstrap must be registered in %s", tc.env)
			require.Equal(t, entity.AuthMethodBootstrap, got.MethodID(),
				"bootstrap must resolve to its own implementation, not the stub")
		})
	}
}

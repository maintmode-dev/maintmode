package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	integrationsvc "github.com/ruko1202/maintmode/internal/services/integration"
)

// provisionLoginRow leaves the fixture's login row the way provisioning does,
// by running it.
func provisionLoginRow(ctx context.Context, t *testing.T, kinds testKinds) {
	t.Helper()

	svc, _ := newServiceFor(t, kinds, nil, config.LoginProviders{kinds.oidc: declared("corp-client")})
	require.NoError(t, svc.Provision(ctx))
}

// A provisioned row belongs to the config file: an admin edit would be
// overwritten at the next restart at best, and at worst would store a secret in
// the database that the feature promises is not there.
//
// Not parallel: it provisions (see provision_test.go).
//
// Proven by mutation: removing the guard from updateWithApply turns the update
// and toggle cases red.
func TestService_ProvisionedRowRefusesAdminWrites(t *testing.T) {
	cases := []struct {
		name  string
		write func(ctx context.Context, svc *integrationsvc.Service, kinds testKinds) error
	}{
		{
			name: "update",
			write: func(ctx context.Context, svc *integrationsvc.Service, kinds testKinds) error {
				_, err := svc.Update(ctx, &entity.UpdateIntegrationCmd{
					Kind: kinds.login, Name: kinds.oidc,
					Secrets: json.RawMessage(`{"client_secret":"admin-typed"}`),
					Actor:   testActor(),
				})
				return err
			},
		},
		{
			// Guard precedence: a config the preset would refuse must still be
			// answered "managed by config", not with the preset's reason -- the
			// operator's remedy is the config file either way.
			name: "update violating the preset",
			write: func(ctx context.Context, svc *integrationsvc.Service, kinds testKinds) error {
				_, err := svc.Update(ctx, &entity.UpdateIntegrationCmd{
					Kind: kinds.login, Name: kinds.oidc,
					Config: json.RawMessage(`{"issuer_url":"https://evil.example","client_id":"x","redirect_uri":"https://app.example/cb"}`),
					Actor:  testActor(),
				})
				return err
			},
		},
		{
			name: "toggle",
			write: func(ctx context.Context, svc *integrationsvc.Service, kinds testKinds) error {
				_, err := svc.Toggle(ctx, &entity.ToggleIntegrationCmd{
					Kind: kinds.login, Name: kinds.oidc, Enabled: lo.ToPtr(false), Actor: testActor(),
				})
				return err
			},
		},
		{
			name: "delete",
			write: func(ctx context.Context, svc *integrationsvc.Service, kinds testKinds) error {
				return svc.Delete(ctx, kinds.login, kinds.oidc, testActor())
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, kinds, mocks := initService(t)
			provisionLoginRow(ctx, t, kinds)
			before, _ := readRow(ctx, t, kinds.login, kinds.oidc)
			auditBefore := len(mocks.audit.Actions())

			err := tc.write(ctx, svc, kinds)

			require.ErrorIs(t, err, apperr.ErrIntegrationNameReserved)
			after, found := readRow(ctx, t, kinds.login, kinds.oidc)
			require.True(t, found, "the row must survive")
			require.Equal(t, before, after, "a refused write changes nothing")
			require.Equal(t, uuid.Nil, mocks.identities.unlinkedFrom, "no identity cascade may run")
			require.Len(t, mocks.audit.Actions(), auditBefore, "a refused write is not audited")
		})
	}
}

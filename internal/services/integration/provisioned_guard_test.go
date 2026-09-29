package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	integrationsvc "github.com/ruko1202/maintmode/internal/services/integration"
)

// provisionedLoginRow creates an ordinary login row and then marks it owned by
// the config file, the way provisioning leaves it. Marked through SQL so the
// guard is tested on its own, independent of Provision.
func provisionedLoginRow(ctx context.Context, t *testing.T, svc *integrationsvc.Service, kinds testKinds) *entity.MaskedIntegration {
	t.Helper()

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.login,
		Name:    kinds.oidc,
		Enabled: lo.ToPtr(true),
		Config:  presetConfig("corp-client"),
		Secrets: json.RawMessage(`{"client_secret":"corp-secret"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`UPDATE integration_settings SET provisioned = true WHERE kind = $1 AND name = $2`, kinds.login, kinds.oidc)
	require.NoError(t, err)

	row, err := svc.GetByKindName(ctx, kinds.login, kinds.oidc)
	require.NoError(t, err)
	require.True(t, row.Provisioned)

	return row
}

// A provisioned row belongs to the config file: an admin edit would be
// overwritten at the next restart at best, and at worst would store a secret in
// the database that the feature promises is not there.
//
// Proven by mutation: removing the guard from updateWithApply turns the update
// and toggle cases red.
func TestService_ProvisionedRowRefusesAdminWrites(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			ctx := context.Background()
			svc, kinds, mocks := initService(t)
			before := provisionedLoginRow(ctx, t, svc, kinds)
			secretBefore := rawStoredSecret(ctx, t, kinds.oidc, "client_secret")
			auditBefore := len(mocks.audit.Actions())

			err := tc.write(ctx, svc, kinds)

			require.ErrorIs(t, err, apperr.ErrIntegrationNameReserved)
			after, getErr := svc.GetByKindName(ctx, kinds.login, kinds.oidc)
			require.NoError(t, getErr, "the row must survive")
			require.Equal(t, before.Enabled, after.Enabled)
			require.JSONEq(t, string(before.Config), string(after.Config))
			require.Equal(t, before.UpdatedAt, after.UpdatedAt)
			require.Equal(t, secretBefore, rawStoredSecret(ctx, t, kinds.oidc, "client_secret"))
			require.Equal(t, uuid.Nil, mocks.identities.unlinkedFrom, "no identity cascade may run")
			require.Len(t, mocks.audit.Actions(), auditBefore, "a refused write is not audited")
		})
	}
}

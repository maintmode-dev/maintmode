package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/config"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
	integrationsvc "github.com/ruko1202/maintmode/internal/services/integration"
)

// None of these are parallel: they provision (see provision_notify_test.go).

// provisionSlack provisions the fixture's slack transport from a config that
// declares it, returning the service that did -- the one holding it in memory.
func provisionSlack(
	ctx context.Context, t *testing.T, kinds testKinds, entry config.NotifyTransportEntry,
) *integrationsvc.Service {
	t.Helper()

	svc, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{kinds.slack: entry})
	require.NoError(t, svc.Provision(ctx))

	return svc
}

// A declared transport is served from this replica's memory, whatever the row
// says: the row is a display copy another replica may have rewritten.
func TestSettings_ServesADeclaredTransportFromMemory(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	svc := provisionSlack(ctx, t, kinds, declaredTransport(
		map[string]any{"api_url": "https://slack.example/api"},
		map[string]string{"bot_token": "cfg-token"}))

	settings, err := svc.Settings(ctx, kinds.notify, kinds.slack)
	require.NoError(t, err)
	require.Equal(t, integrationkinds.SlackSettings{BotToken: "cfg-token", APIURL: "https://slack.example/api"}, settings)

	_, err = db.ExecContext(ctx,
		`UPDATE integration_settings SET enabled = false, config = '{}' WHERE kind = $1 AND name = $2`,
		kinds.notify, kinds.slack)
	require.NoError(t, err)

	settings, err = svc.Settings(ctx, kinds.notify, kinds.slack)
	require.NoError(t, err, "memory wins over the row")
	require.Equal(t, "cfg-token", settings.(integrationkinds.SlackSettings).BotToken)
}

// Every notify kind comes out of memory with its secret merged in -- email's
// password included, which OTP sign-in and invitations depend on -- and an
// unauthenticated relay with none.
func TestSettings_ServesEveryDeclaredKind(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	_, relayKinds, _ := initService(t)

	svc, _ := newServiceDeclaring(t, kinds, nil, nil, config.NotifyTransportEntries{
		kinds.telegram: declaredTransport(nil, map[string]string{"bot_token": "tg-token"}),
		kinds.email: declaredTransport(map[string]any{
			"host": "smtp.example.com", "port": 587, "from": "maint@example.com",
			"username": "maint", "tls_policy": "mandatory",
		}, map[string]string{"password": "smtp-password"}),
	})
	require.NoError(t, svc.Provision(ctx))

	telegram, err := svc.Settings(ctx, kinds.notify, kinds.telegram)
	require.NoError(t, err)
	require.Equal(t, "tg-token", telegram.(integrationkinds.TelegramSettings).BotToken)

	email, err := svc.Settings(ctx, kinds.notify, kinds.email)
	require.NoError(t, err)
	require.Equal(t, integrationkinds.EmailSettings{
		Host: "smtp.example.com", Port: 587, From: "maint@example.com",
		Username: "maint", Password: "smtp-password", TLSPolicy: "mandatory",
	}, email)

	relaySvc, _ := newServiceDeclaring(t, relayKinds, nil, nil, config.NotifyTransportEntries{
		relayKinds.email: declaredTransport(map[string]any{
			"host": "relay.internal", "port": 25, "from": "maint@example.com", "tls_policy": "none",
		}, nil),
	})
	require.NoError(t, relaySvc.Provision(ctx))

	relayed, err := relaySvc.Settings(ctx, relayKinds.notify, relayKinds.email)
	require.NoError(t, err)
	require.Empty(t, relayed.(integrationkinds.EmailSettings).Password)
	require.Equal(t, "relay.internal", relayed.(integrationkinds.EmailSettings).Host)
}

func TestSettings_PinnedOffTransportIsDisabled(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	svc := provisionSlack(ctx, t, kinds, config.NotifyTransportEntry{
		ManagedBy: config.ManagedByConfig, Enabled: lo.ToPtr(false),
	})

	_, err := svc.Settings(ctx, kinds.notify, kinds.slack)

	require.ErrorIs(t, err, apperr.ErrIntegrationDisabled)
}

// A provisioned row this replica does not declare -- another replica's config
// put it there -- has no secret in the database, so it cannot be served here.
// Without the check it would build a transport with an empty token and report
// it healthy: Settings does not validate.
func TestSettings_UndeclaredProvisionedTransport(t *testing.T) {
	ctx := context.Background()
	_, kinds, _ := initService(t)
	provisionSlack(ctx, t, kinds, declaredTransport(nil, map[string]string{"bot_token": "cfg-token"}))
	// This replica's config declares nothing. It is not provisioned: its
	// Provision would release the row, and the point is a replica that
	// started before the row was provisioned.
	here, _ := newServiceDeclaring(t, kinds, nil, nil, nil)

	_, err := here.Settings(ctx, kinds.notify, kinds.slack)
	require.ErrorIs(t, err, apperr.ErrIntegrationUnreadable, "enabled elsewhere, unservable here")

	_, err = db.ExecContext(ctx,
		`UPDATE integration_settings SET enabled = false WHERE kind = $1 AND name = $2`, kinds.notify, kinds.slack)
	require.NoError(t, err)

	_, err = here.Settings(ctx, kinds.notify, kinds.slack)
	require.ErrorIs(t, err, apperr.ErrIntegrationDisabled)
}

// The guard is category-neutral; this pins it for a notify row.
func TestService_ProvisionedTransportRefusesAdminWrites(t *testing.T) {
	cases := []struct {
		name  string
		write func(ctx context.Context, svc *integrationsvc.Service, kinds testKinds) error
	}{
		{
			name: "update",
			write: func(ctx context.Context, svc *integrationsvc.Service, kinds testKinds) error {
				_, err := svc.Update(ctx, &entity.UpdateIntegrationCmd{
					Kind: kinds.notify, Name: kinds.slack,
					Secrets: json.RawMessage(`{"bot_token":"admin-typed"}`),
					Actor:   testActor(),
				})
				return err
			},
		},
		{
			name: "toggle",
			write: func(ctx context.Context, svc *integrationsvc.Service, kinds testKinds) error {
				_, err := svc.Toggle(ctx, &entity.ToggleIntegrationCmd{
					Kind: kinds.notify, Name: kinds.slack, Enabled: lo.ToPtr(false), Actor: testActor(),
				})
				return err
			},
		},
		{
			name: "delete",
			write: func(ctx context.Context, svc *integrationsvc.Service, kinds testKinds) error {
				return svc.Delete(ctx, kinds.notify, kinds.slack, testActor())
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, kinds, _ := initService(t)
			provisionSlack(ctx, t, kinds, declaredTransport(nil, map[string]string{"bot_token": "cfg-token"}))
			before, _ := readRow(ctx, t, kinds.notify, kinds.slack)

			err := tc.write(ctx, svc, kinds)

			require.ErrorIs(t, err, apperr.ErrIntegrationNameReserved)
			after, found := readRow(ctx, t, kinds.notify, kinds.slack)
			require.True(t, found)
			require.Equal(t, before, after, "a refused write changes nothing")
		})
	}
}

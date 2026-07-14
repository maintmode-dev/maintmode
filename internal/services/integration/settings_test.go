package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// TestSettings_ErrorTaxonomy pins the RUK-200 error contract of the resolve
// seam — the single source both the delivery path and the transport_status
// read model classify from:
//
//   - missing row      → ErrIntegrationNotConfigured, which must ALSO satisfy
//     errors.Is(_, ErrIntegrationDisabled) so the dispatch drop keeps working;
//   - disabled row     → ErrIntegrationDisabled but NOT NotConfigured;
//   - enabled + broken → ErrIntegrationUnreadable (never Disabled — delivery
//     must retry, not silently drop an operational fault).
func TestSettings_ErrorTaxonomy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("missing is not_configured wrapping disabled", func(t *testing.T) {
		t.Parallel()
		svc, _, _ := initService(t)

		_, err := svc.Settings(ctx, "ghost-"+xuuid.NewString())
		require.ErrorIs(t, err, apperr.ErrIntegrationNotConfigured)
		require.ErrorIs(t, err, apperr.ErrIntegrationDisabled,
			"the dispatch drop contract relies on this wrap")
		require.NotErrorIs(t, err, apperr.ErrIntegrationUnreadable)
	})

	t.Run("disabled is disabled but not not_configured", func(t *testing.T) {
		t.Parallel()
		svc, kinds, _ := initService(t)
		_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
			Kind:    kinds.slack,
			Enabled: lo.ToPtr(false),
			Config:  json.RawMessage(`{"api_url":"https://slack.test"}`),
			Secrets: secretsJSON(t, map[string]string{"bot_token": "xoxb-disabled"}),
			Actor:   testActor(),
		})
		require.NoError(t, err)

		_, err = svc.Settings(ctx, kinds.slack)
		require.ErrorIs(t, err, apperr.ErrIntegrationDisabled)
		require.NotErrorIs(t, err, apperr.ErrIntegrationNotConfigured)
	})

	// The RUK-200 production incident: the KEK a DEK was wrapped with is no
	// longer known to the keyring (rollback across a rotation phase).
	t.Run("unknown KEK is unreadable, not disabled", func(t *testing.T) {
		t.Parallel()
		svc, kinds, _ := initService(t)
		createEnabledSlack(ctx, t, svc, kinds.slack)

		_, err := db.ExecContext(ctx,
			`UPDATE data_keys SET kek_id = 'local-kms://gone'
			 WHERE id = (SELECT dek_id FROM integration_settings WHERE kind = $1)`, kinds.slack)
		require.NoError(t, err)

		_, err = svc.Settings(ctx, kinds.slack)
		require.ErrorIs(t, err, apperr.ErrIntegrationUnreadable)
		require.NotErrorIs(t, err, apperr.ErrIntegrationDisabled,
			"an operational fault must not be silently dropped as a drop")
	})

	t.Run("corrupt envelope is unreadable", func(t *testing.T) {
		t.Parallel()
		svc, kinds, _ := initService(t)
		createEnabledSlack(ctx, t, svc, kinds.slack)

		_, err := db.ExecContext(ctx,
			`UPDATE integration_settings
			 SET secrets = jsonb_set(secrets, '{bot_token}', to_jsonb('Y29ycnVwdA=='::text))
			 WHERE kind = $1`, kinds.slack)
		require.NoError(t, err)

		_, err = svc.Settings(ctx, kinds.slack)
		require.ErrorIs(t, err, apperr.ErrIntegrationUnreadable)
	})

	t.Run("kind missing from code registry is unreadable", func(t *testing.T) {
		t.Parallel()
		svc, kinds, _ := initService(t)
		createEnabledSlack(ctx, t, svc, kinds.slack)

		ghost := kinds.slack + "-ghost"
		_, err := db.ExecContext(ctx,
			`UPDATE integration_settings SET kind = $1 WHERE kind = $2`, ghost, kinds.slack)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.Exec(`DELETE FROM integration_settings WHERE kind = $1`, ghost)
		})

		_, err = svc.Settings(ctx, ghost)
		require.ErrorIs(t, err, apperr.ErrIntegrationUnreadable)
	})

	t.Run("healthy enabled integration resolves", func(t *testing.T) {
		t.Parallel()
		svc, kinds, _ := initService(t)
		createEnabledSlack(ctx, t, svc, kinds.slack)

		settings, err := svc.Settings(ctx, kinds.slack)
		require.NoError(t, err)
		require.NotNil(t, settings)
	})
}

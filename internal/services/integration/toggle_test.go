package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestService_Toggle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, mocks := initService(t)

	created, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "t"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	require.True(t, created.Enabled)
	secretBefore := rawStoredSecret(ctx, t, kinds.slack, "bot_token")

	toggled, err := svc.Toggle(ctx, &entity.ToggleIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(false),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	require.False(t, toggled.Enabled)

	// Toggle only flips enabled: config and secret bytes are untouched.
	got, err := svc.GetByKind(ctx, kinds.slack)
	require.NoError(t, err)
	require.JSONEq(t, `{"api_url":"https://a.test"}`, string(got.Config))
	require.Equal(t, secretBefore, rawStoredSecret(ctx, t, kinds.slack, "bot_token"),
		"toggle must not re-write the secret")

	// Toggle publishes an IntegrationUpdated audit event (the create above also
	// published an IntegrationCreated on the same spy).
	upd := lastUpdated(t, mocks.audit.Actions())
	require.False(t, upd.Enabled)
}

func TestService_ToggleWithoutActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)
	_, err := svc.Toggle(ctx, &entity.ToggleIntegrationCmd{
		Kind: kinds.slack, Enabled: lo.ToPtr(false), Actor: nil,
	})
	require.ErrorIs(t, err, apperr.ErrValidation, "nil actor must be a typed error, not a panic")
}

// Update and Toggle must advance updated_at; an earlier version of the store wrote
// back the stale timestamp so it silently froze. The store stamps updated_at to the
// app clock on every write, so consecutive writes are monotonic — assert against a
// prior write's stamp (same clock source) rather than create's DB-default now(),
// which a different clock source makes an unreliable baseline.
func TestService_UpdateAndToggleAdvanceUpdatedAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "t"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	updated, err := svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://b.test"}`),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	toggled, err := svc.Toggle(ctx, &entity.ToggleIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(false),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	require.True(t, toggled.UpdatedAt.After(updated.UpdatedAt),
		"Toggle must advance updated_at past the prior write (was %s, now %s)",
		updated.UpdatedAt, toggled.UpdatedAt)
}

func TestService_ToggleNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Toggle(ctx, &entity.ToggleIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(false),
		Actor:   testActor(),
	})
	require.ErrorIs(t, err, apperr.ErrIntegrationNotFound)
}

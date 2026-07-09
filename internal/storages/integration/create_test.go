package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestStore_CreateAndGetByKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dekID := seedDEK(ctx, t)
	setting := newSetting(t, dekID)

	created, err := store.Create(ctx, setting)
	require.NoError(t, err)
	require.NotEqual(t, entity.IntegrationSetting{}.ID, created.ID, "id assigned by DB")
	require.False(t, created.CreatedAt.IsZero())
	require.Equal(t, dekID, created.DEKID)

	got, err := store.GetByKind(ctx, setting.Kind)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	// jsonb round-trip preserves config (raw JSON) and the secrets map.
	require.JSONEq(t, string(setting.Config), string(got.Config))
	require.Equal(t, setting.Secrets, got.Secrets)
}

func TestStore_ConfigRawJSONRoundTrips(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	setting := newSetting(t, seedDEK(ctx, t))
	// Config is stored verbatim as raw JSON: numbers, nesting, and unicode must
	// survive the jsonb round-trip so a kind's json.Unmarshal sees exactly what
	// was written.
	setting.Config = json.RawMessage(`{"port":587,"timeout":"30s","nested":{"k":"零"}}`)
	// Multi-key secret with base64/unicode values to prove byte-faithful codec.
	setting.Secrets = map[string]string{"bot_token": "a+b/c=", "webhook_secret": "零"}

	created, err := store.Create(ctx, setting)
	require.NoError(t, err)

	got, err := store.GetByKind(ctx, created.Kind)
	require.NoError(t, err)
	require.JSONEq(t, string(setting.Config), string(got.Config))
	require.Equal(t, setting.Secrets, got.Secrets)
}

func TestStore_CreateDuplicateKindConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dekID := seedDEK(ctx, t)
	setting := newSetting(t, dekID)

	_, err := store.Create(ctx, setting)
	require.NoError(t, err)

	dup := *setting // same kind
	_, err = store.Create(ctx, &dup)
	require.ErrorIs(t, err, apperr.ErrIntegrationConflict)
}

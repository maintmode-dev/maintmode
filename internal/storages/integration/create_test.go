package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestStore_CreateAndGetByKindName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dekID := seedDEK(ctx, t)
	setting := newSetting(t, dekID)

	created, err := store.Create(ctx, setting)
	require.NoError(t, err)
	require.NotEqual(t, entity.IntegrationSetting{}.ID, created.ID, "id assigned by DB")
	require.False(t, created.CreatedAt.IsZero())
	require.Equal(t, dekID, created.DEKID)

	got, err := store.GetByKindName(ctx, setting.Kind, setting.Name)
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

	got, err := store.GetByKindName(ctx, created.Kind, created.Name)
	require.NoError(t, err)
	require.JSONEq(t, string(setting.Config), string(got.Config))
	require.Equal(t, setting.Secrets, got.Secrets)
}

// The whole point of the migration: one kind, several named instances. Under
// the old UNIQUE (kind) the second Create here conflicted.
func TestStore_SameKindDifferentNamesCoexist(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dekID := seedDEK(ctx, t)

	first := newSetting(t, dekID)
	first.Name = "google-" + xuuid.NewString()
	_, err := store.Create(ctx, first)
	require.NoError(t, err)

	second := *first
	second.Name = "keycloak-" + xuuid.NewString()
	_, err = store.Create(ctx, &second)
	require.NoError(t, err)

	// Each name addresses its own row, rather than one shadowing the other.
	got, err := store.GetByKindName(ctx, first.Kind, second.Name)
	require.NoError(t, err)
	require.Equal(t, second.Name, got.Name)

	got, err = store.GetByKindName(ctx, first.Kind, first.Name)
	require.NoError(t, err)
	require.Equal(t, first.Name, got.Name)
}

// Guards the silent-drop seam: every layer between the column and the entity
// lists its fields by name, so a mapper that forgets Name would still compile,
// still pass every other test, and quietly store the column default. Mutation
// check: remove Name from toDB and this is the test that goes red.
func TestStore_NamePersistsVerbatim(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	setting := newSetting(t, seedDEK(ctx, t))
	name := "corporate-idp-" + xuuid.NewString()
	setting.Name = name

	created, err := store.Create(ctx, setting)
	require.NoError(t, err)
	require.Equal(t, name, created.Name, "name must round-trip, not fall back to the column default")

	got, err := store.GetByKindName(ctx, setting.Kind, name)
	require.NoError(t, err)
	require.Equal(t, name, got.Name)
}

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/integrationkinds"
)

func TestStore_GetByKindNameNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, err := store.GetByKindName(ctx, integrationkinds.CategoryNotify, "nonexistent-"+t.Name())
	require.ErrorIs(t, err, apperr.ErrIntegrationNotFound)
}

func TestStore_CreateWithEmptyConfigAndSecrets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dekID := seedDEK(ctx, t)
	setting := newSetting(t, dekID)
	setting.Config = nil
	setting.Secrets = nil

	created, err := store.Create(ctx, setting)
	require.NoError(t, err)

	got, err := store.GetByKindName(ctx, created.Kind, created.Name)
	require.NoError(t, err)
	// Nil config round-trips as the empty JSON object "{}" so a kind's
	// json.Unmarshal always gets a valid object body; nil secrets round-trip as
	// an empty (non-nil) map decoded once by the store mapper.
	require.JSONEq(t, `{}`, string(got.Config))
	require.NotNil(t, got.Secrets)
	require.Empty(t, got.Secrets)
}

func TestStore_GetForUpdateReads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dekID := seedDEK(ctx, t)
	created, err := store.Create(ctx, newSetting(t, dekID))
	require.NoError(t, err)

	// A bare GetForUpdateByKindName outside an explicit tx still reads the row; the
	// lock semantics are exercised by the service layer's WithinTx in Task 9.
	got, err := store.GetForUpdateByKindName(ctx, created.Kind, created.Name)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

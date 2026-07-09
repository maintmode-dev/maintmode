package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
)

func TestStore_UpdatePreservesCreatedAndKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dekID := seedDEK(ctx, t)
	created, err := store.Create(ctx, newSetting(t, dekID))
	require.NoError(t, err)

	editor := lo.ToPtr(uuid.New())
	created.Enabled = false
	created.Config = json.RawMessage(`{"api_url":"https://changed.test"}`)
	created.Secrets = map[string]string{"bot_token": "newcipher"}
	created.UpdatedByUserID = editor

	updated, err := store.Update(ctx, created)
	require.NoError(t, err)
	require.False(t, updated.Enabled)
	require.JSONEq(t, string(created.Config), string(updated.Config))
	require.Equal(t, created.Secrets, updated.Secrets)
	require.Equal(t, created.Kind, updated.Kind, "kind is immutable")
	require.Equal(t, created.CreatedByUserID, updated.CreatedByUserID, "original author preserved")
	require.Equal(t, editor, updated.UpdatedByUserID)
}

func TestStore_UpdateNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ghost := newSetting(t, seedDEK(ctx, t))
	ghost.ID = uuid.New() // never inserted

	_, err := store.Update(ctx, ghost)
	require.ErrorIs(t, err, apperr.ErrIntegrationNotFound)
}

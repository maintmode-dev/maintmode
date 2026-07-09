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

func TestService_GetNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, _ := initService(t)
	_, err := svc.GetByKind(ctx, "telegram-missing-"+xuuid.NewString())
	require.ErrorIs(t, err, apperr.ErrIntegrationNotFound)
}

// List is the one remaining read path; like GetByKind it must surface only the
// is-set view — neither the plaintext nor the stored ciphertext may appear
// anywhere in the returned items.
func TestService_ListMasksSecrets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	const plaintext = "xoxb-list-secret"
	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://slack.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": plaintext}),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	ciphertext := rawStoredSecret(ctx, t, kinds.slack, "bot_token")

	list, err := svc.List(ctx)
	require.NoError(t, err)

	item, found := lo.Find(list, func(m *entity.MaskedIntegration) bool { return m.Kind == kinds.slack })
	require.True(t, found, "the created integration must be listed")
	require.Equal(t, map[string]bool{"bot_token": true}, item.SecretsSet)

	// The full serialized view carries no secret in any form — this catches a
	// field added to MaskedIntegration later that accidentally leaks.
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	require.NotContains(t, string(raw), plaintext, "List must never surface the plaintext secret")
	require.NotContains(t, string(raw), ciphertext, "List must never surface the stored ciphertext")
}

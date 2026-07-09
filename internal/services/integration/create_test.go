package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/samber/lo"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/audit"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestService_CreateEncryptsSecretAndMasksOnRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, _ := initService(t)

	masked, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://slack.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "xoxb-super-secret"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	// Read path returns a mask, never the secret value.
	require.Equal(t, map[string]bool{"bot_token": true}, masked.SecretsSet)
	require.JSONEq(t, `{"api_url":"https://slack.test"}`, string(masked.Config))

	// The stored secret is ciphertext, not the plaintext input.
	stored := rawStoredSecret(ctx, t, kinds.slack, "bot_token")
	require.NotEmpty(t, stored)
	require.NotContains(t, stored, "xoxb-super-secret", "stored secret must be encrypted, not plaintext")

	// GetByKind also masks.
	got, err := svc.GetByKind(ctx, kinds.slack)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"bot_token": true}, got.SecretsSet)
}

func TestService_CreateStoresConfigVerbatimAndSecretSeparately(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, _ := initService(t)

	// The real secret goes in Secrets; config is stored raw as supplied. The
	// secret value from Secrets must never appear in the returned config, and the
	// config JSON is preserved byte-equivalent (opaque to the service).
	masked, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://slack.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "xoxb-real"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"api_url":"https://slack.test"}`, string(masked.Config),
		"config is stored and returned verbatim")
	require.NotContains(t, string(masked.Config), "xoxb-real",
		"the secret value must never appear in config")
	require.True(t, masked.SecretsSet["bot_token"], "the secret is tracked via secrets_set")
}

func TestService_CreateUnknownKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, _, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    "jira-" + xuuid.NewString(),
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "t"}),
		Actor:   testActor(),
	})
	require.ErrorIs(t, err, apperr.ErrUnknownIntegrationKind)
}

func TestService_CreateInvalidConfigRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, _ := initService(t)

	// Missing bot_token secret -> kind validation fails.
	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{}`),
		Secrets: secretsJSON(t, map[string]string{}),
		Actor:   testActor(),
	})
	require.ErrorIs(t, err, apperr.ErrValidation)
}

func TestService_CreateWithoutEnabledRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: nil, // omitted
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "t"}),
		Actor:   testActor(),
	})
	require.ErrorIs(t, err, apperr.ErrValidation)
}

func TestService_CreateWithoutActorRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "t"}),
		Actor:   nil,
	})
	require.ErrorIs(t, err, apperr.ErrValidation)
}

func TestService_CreatePublishesAuditWithoutSecret(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, mocks := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "top-secret-token"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	require.Len(t, mocks.audit.Actions(), 1)
	created, ok := mocks.audit.Actions()[0].(audit.IntegrationCreated)
	require.True(t, ok, "an IntegrationCreated action is published")
	require.Equal(t, kinds.slack, created.Kind)

	// The audit action carries no secret value in any field.
	require.NotContains(t, fmt.Sprintf("%+v", created), "top-secret-token",
		"secret must never enter the audit payload")
}

// The audit action records the acting admin, not just the kind — actor threading
// is the point of the RUK-182 audit design.
func TestService_CreatePublishesAuditWithActor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, mocks := initService(t)

	actor := testActor()
	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "t"}),
		Actor:   actor,
	})
	require.NoError(t, err)

	require.Len(t, mocks.audit.Actions(), 1)
	created, ok := mocks.audit.Actions()[0].(audit.IntegrationCreated)
	require.True(t, ok)
	require.Equal(t, actor, created.Actor, "the audit action must carry the acting admin")
}

// Creating a second row for an existing kind is a conflict at the service layer
// (the store's UNIQUE(kind) surfaces as ErrIntegrationConflict).
func TestService_CreateDuplicateKindConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, _ := initService(t)

	cmd := &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "t"}),
		Actor:   testActor(),
	}
	_, err := svc.Create(ctx, cmd)
	require.NoError(t, err)

	_, err = svc.Create(ctx, cmd)
	require.ErrorIs(t, err, apperr.ErrIntegrationConflict)
}

// A secret submitted under its well-known key inside the plaintext config (instead
// of secrets) is rejected: config is stored and returned verbatim, so such a value
// would sit unencrypted in the DB and leak on every GET. The guard fires per kind
// on its own SecretKeys() — slack's bot_token, email's password.
func TestService_CreateRejectsSecretKeyInConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, _ := initService(t)

	cases := []struct {
		name      string
		kind      string
		config    string
		secretKey string
	}{
		{"slack bot_token in config", kinds.slack, `{"bot_token":"xoxb-leaked","api_url":"https://a.test"}`, "bot_token"},
		{"email password in config", kinds.email, `{"host":"smtp.test","from":"a@b.c","password":"leaked"}`, "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
				Kind:    tc.kind,
				Enabled: lo.ToPtr(true),
				Config:  json.RawMessage(tc.config),
				Secrets: secretsJSON(t, map[string]string{}),
				Actor:   testActor(),
			})
			require.ErrorIs(t, err, apperr.ErrValidation,
				"a secret key in config must be rejected as validation error")
			require.ErrorContains(t, err, tc.secretKey,
				"the error must name the offending key so the caller can fix the section")
			require.NotContains(t, err.Error(), "leaked",
				"the secret VALUE must never appear in the error")
		})
	}
}

// A config key that is NOT one of the kind's SecretKeys() is left alone — the
// guard only classifies exact secret-name matches; an arbitrary plaintext field
// is legitimate config.
func TestService_CreateAllowsNonSecretConfigKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, _ := initService(t)

	// "api_url" is plaintext config; only "bot_token" would be rejected.
	masked, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test","timeout":"10s"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "xoxb-real"}),
		Actor:   testActor(),
	})
	require.NoError(t, err, "non-secret config keys must not trip the secret-in-config guard")
	require.JSONEq(t, `{"api_url":"https://a.test","timeout":"10s"}`, string(masked.Config))
}

// The same guard protects the update path: an edit cannot smuggle a secret into
// the plaintext config either.
func TestService_UpdateRejectsSecretKeyInConfig(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "xoxb-real"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"bot_token":"xoxb-leaked","api_url":"https://a.test"}`),
		Secrets: secretIntentsJSON(t, map[string]*string{}),
		Actor:   testActor(),
	})
	require.ErrorIs(t, err, apperr.ErrValidation)
	require.ErrorContains(t, err, "bot_token")
}

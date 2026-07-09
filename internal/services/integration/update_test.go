package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/samber/lo"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
)

func TestService_UpdateKeepsAbsentSecret(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "original-token"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	before := rawStoredSecret(ctx, t, kinds.slack, "bot_token")

	// Update config only, no secret in the command.
	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://b.test"}`),
		Secrets: secretIntentsJSON(t, map[string]*string{}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	got, err := svc.GetByKind(ctx, kinds.slack)
	require.NoError(t, err)
	require.JSONEq(t, `{"api_url":"https://b.test"}`, string(got.Config), "config updated")
	require.Equal(t, map[string]bool{"bot_token": true}, got.SecretsSet, "secret retained")

	// A config-only update carries the stored ciphertext forward verbatim — no
	// decrypt/re-encrypt — so the persisted bytes are byte-identical.
	after := rawStoredSecret(ctx, t, kinds.slack, "bot_token")
	require.Equal(t, before, after, "unchanged secret is carried over, not re-encrypted")
}

// A secret can be explicitly cleared (JSON null / empty) on update — the review
// gap that made it impossible to drop SMTP auth without editing the DB by hand.
// Here an authenticated email integration is switched to an open relay by clearing
// the password (and dropping username from config in the same edit).
func TestService_UpdateClearsSecret(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.email,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"host":"smtp.test","from":"a@b.c","username":"u","tls_policy":"none"}`),
		Secrets: secretsJSON(t, map[string]string{"password": "pw"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	// Clear the password (null) and drop username in the same edit, so the
	// half-credential invariant still holds — the result is a valid open relay.
	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.email,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"host":"smtp.test","from":"a@b.c","tls_policy":"none"}`),
		Secrets: secretIntentsJSON(t, map[string]*string{"password": nil}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	got, err := svc.GetByKind(ctx, kinds.email)
	require.NoError(t, err)
	require.NotContains(t, got.SecretsSet, "password", "the cleared secret must be gone from secrets_set")

	// The secrets column no longer holds a password key at all.
	var present bool
	require.NoError(t, db.QueryRowxContext(ctx,
		`SELECT secrets ? 'password' FROM integration_settings WHERE kind = $1`, kinds.email).Scan(&present))
	require.False(t, present, "the cleared secret must be removed from the stored jsonb")
}

// Clearing a required secret is rejected by kind validation, not silently
// applied — clearing slack's bot_token leaves an invalid integration.
func TestService_UpdateClearingRequiredSecretRejected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "tok"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretIntentsJSON(t, map[string]*string{"bot_token": nil}),
		Actor:   testActor(),
	})
	require.ErrorIs(t, err, apperr.ErrValidation, "clearing a required secret must fail validation")

	// And the stored secret is untouched (the failed tx rolled back).
	require.Equal(t, "tok", decryptStoredSecret(ctx, t, kinds.slack, "bot_token"))
}

func TestService_UpdatePreservesAbsentSecretPlaintext(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "original-token"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://b.test"}`),
		Secrets: secretIntentsJSON(t, map[string]*string{}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	// The decrypted secret must still be the original — the invariant the
	// mask-only read path cannot prove on its own.
	require.Equal(t, "original-token", decryptStoredSecret(ctx, t, kinds.slack, "bot_token"))
}

func TestService_UpdateChangesSecret(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "old"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretIntentsJSON(t, map[string]*string{"bot_token": lo.ToPtr("new-token")}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	stored := rawStoredSecret(ctx, t, kinds.slack, "bot_token")
	require.NotContains(t, stored, "new-token", "new secret stored encrypted")
}

func TestService_UpdatePublishesAuditAndReusesDEK(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, mocks := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "tok"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	dekBefore := rawStoredDEKID(ctx, t, kinds.slack)

	_, err = svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://b.test"}`),
		Secrets: secretIntentsJSON(t, map[string]*string{"bot_token": lo.ToPtr("new-tok")}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	// Update reuses the existing DEK — never repoints ciphertext at a new key.
	require.Equal(t, dekBefore, rawStoredDEKID(ctx, t, kinds.slack), "update must reuse the DEK")

	// Update publishes IntegrationUpdated with no secret in the payload. The
	// create above also published an IntegrationCreated on the same spy, so filter
	// to the update event rather than asserting a bare count.
	upd := lastUpdated(t, mocks.audit.Actions())
	require.Equal(t, kinds.slack, upd.Kind)
	require.NotContains(t, fmt.Sprintf("%+v", upd), "new-tok")
}

// Update/Toggle on a never-created kind surface ErrIntegrationNotFound from the
// FOR UPDATE read, not a nil-deref or a silent create.
func TestService_UpdateNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://a.test"}`),
		Secrets: secretIntentsJSON(t, map[string]*string{"bot_token": lo.ToPtr("t")}),
		Actor:   testActor(),
	})
	require.ErrorIs(t, err, apperr.ErrIntegrationNotFound)
}

// PATCH semantics: fields the caller omits keep their stored values — an update
// that sends nothing but the kind must be a no-op for enabled, config, and
// secrets alike.
func TestService_UpdateOmittedFieldsKeepStored(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://keep.me"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "keep-token"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	// Everything omitted: nil Enabled, nil Config, nil Secrets.
	updated, err := svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:  kinds.slack,
		Actor: testActor(),
	})
	require.NoError(t, err)

	require.True(t, updated.Enabled, "omitted enabled must keep the stored flag")
	require.JSONEq(t, `{"api_url":"https://keep.me"}`, string(updated.Config), "omitted config must keep the stored config")
	require.True(t, updated.SecretsSet["bot_token"], "omitted secrets must keep the stored secret")
	require.Equal(t, "keep-token", decryptStoredSecret(ctx, t, kinds.slack, "bot_token"),
		"the stored ciphertext must be untouched")
}

// An explicit empty object is a real value, not an omission: it replaces the
// stored config wholesale — the caller's way to clear config.
func TestService_UpdateExplicitEmptyConfigReplaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, kinds, _ := initService(t)

	_, err := svc.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    kinds.slack,
		Enabled: lo.ToPtr(true),
		Config:  json.RawMessage(`{"api_url":"https://old.example"}`),
		Secrets: secretsJSON(t, map[string]string{"bot_token": "t"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	updated, err := svc.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:   kinds.slack,
		Config: json.RawMessage(`{}`),
		Actor:  testActor(),
	})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(updated.Config), "explicit {} must replace the stored config")
}

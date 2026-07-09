package transportresolver_test

import (
	"context"
	"testing"

	"github.com/samber/lo"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestResolver_BuildsAndCaches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := initResolver(t)

	_, err := h.registry.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    h.kind,
		Enabled: lo.ToPtr(true),
		Secrets: secretsJSON(t, map[string]string{"token": "sekret"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	tr, err := h.resolve(ctx, h.kind)
	require.NoError(t, err)
	require.NotNil(t, tr)
	require.Equal(t, "sekret", tr.(resolvableTransport).Token(),
		"resolve must wire the decrypted plaintext into the transport")

	// A second resolve is a cache hit: deleting the row from under it must not
	// change the result (the store is not consulted on a hit).
	_, err = db.ExecContext(ctx, `DELETE FROM integration_settings WHERE kind = $1`, h.kind)
	require.NoError(t, err)
	tr2, err := h.resolve(ctx, h.kind)
	require.NoError(t, err, "cache hit must not hit the store")
	require.NotNil(t, tr2)
}

func TestResolver_DropsWhenDisabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := initResolver(t)

	_, err := h.registry.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    h.kind,
		Enabled: lo.ToPtr(false),
		Secrets: secretsJSON(t, map[string]string{"token": "sekret"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	_, err = h.resolve(ctx, h.kind)
	require.ErrorIs(t, err, apperr.ErrIntegrationDisabled, "disabled integration drops")
}

func TestResolver_UnknownKindDisabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := initResolver(t)
	_, err := h.resolve(ctx, "never-configured-"+xuuid.NewString())
	require.ErrorIs(t, err, apperr.ErrIntegrationDisabled, "unconfigured kind drops best-effort")
}

func TestResolver_WriteInvalidatesCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := initResolver(t)

	_, err := h.registry.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    h.kind,
		Enabled: lo.ToPtr(true),
		Secrets: secretsJSON(t, map[string]string{"token": "sekret"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	_, err = h.resolve(ctx, h.kind) // warms the cache
	require.NoError(t, err)

	// Toggle off; the write must invalidate the cache so the next resolve drops.
	_, err = h.registry.Toggle(ctx, &entity.ToggleIntegrationCmd{
		Kind: h.kind, Enabled: lo.ToPtr(false), Actor: testActor(),
	})
	require.NoError(t, err)

	_, err = h.resolve(ctx, h.kind)
	require.ErrorIs(t, err, apperr.ErrIntegrationDisabled,
		"a write must invalidate the cache so a disable takes effect immediately")
}

func TestResolver_RebuildsWithRotatedSecret(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := initResolver(t)

	_, err := h.registry.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    h.kind,
		Enabled: lo.ToPtr(true),
		Secrets: secretsJSON(t, map[string]string{"token": "old"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	tr, err := h.resolve(ctx, h.kind) // warms cache with "old"
	require.NoError(t, err)
	require.Equal(t, "old", tr.(resolvableTransport).Token())

	// Rotate the secret; the update must invalidate the cache so the next resolve
	// rebuilds a transport bound to the NEW secret — the "rotate without restart"
	// contract that justifies the runtime seam.
	_, err = h.registry.Update(ctx, &entity.UpdateIntegrationCmd{
		Kind:    h.kind,
		Enabled: lo.ToPtr(true),
		Secrets: secretIntentsJSON(t, map[string]*string{"token": lo.ToPtr("new")}),
		Actor:   testActor(),
	})
	require.NoError(t, err)

	tr2, err := h.resolve(ctx, h.kind)
	require.NoError(t, err)
	require.Equal(t, "new", tr2.(resolvableTransport).Token(),
		"resolve must rebuild with the rotated secret after invalidation")
}

func TestResolver_ReenableRebuilds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	h := initResolver(t)

	_, err := h.registry.Create(ctx, &entity.CreateIntegrationCmd{
		Kind:    h.kind,
		Enabled: lo.ToPtr(true),
		Secrets: secretsJSON(t, map[string]string{"token": "t"}),
		Actor:   testActor(),
	})
	require.NoError(t, err)
	_, err = h.resolve(ctx, h.kind) // warm
	require.NoError(t, err)

	_, err = h.registry.Toggle(ctx, &entity.ToggleIntegrationCmd{Kind: h.kind, Enabled: lo.ToPtr(false), Actor: testActor()})
	require.NoError(t, err)
	_, err = h.resolve(ctx, h.kind)
	require.ErrorIs(t, err, apperr.ErrIntegrationDisabled)

	// Re-enable: the write invalidates and the next resolve builds successfully.
	_, err = h.registry.Toggle(ctx, &entity.ToggleIntegrationCmd{Kind: h.kind, Enabled: lo.ToPtr(true), Actor: testActor()})
	require.NoError(t, err)
	tr, err := h.resolve(ctx, h.kind)
	require.NoError(t, err)
	require.NotNil(t, tr)
}

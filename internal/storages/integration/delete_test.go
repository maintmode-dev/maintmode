package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

// Delete addresses one row, and which one is decided by a single predicate.
// Widening it to the kind would take every instance of that kind with it --
// silently, since the caller already checked that no accounts are linked and
// the ciphertext is unrecoverable afterwards.
func TestStore_DeleteRemovesOnlyTheNamedRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dekID := seedDEK(ctx, t)

	keep := newSetting(t, dekID)
	keep.Name = "google-" + xuuid.NewString()
	_, err := store.Create(ctx, keep)
	require.NoError(t, err)

	drop := *keep
	drop.Name = "keycloak-" + xuuid.NewString()
	_, err = store.Create(ctx, &drop)
	require.NoError(t, err)

	require.NoError(t, store.Delete(ctx, keep.Kind, drop.Name))

	_, err = store.GetByKindName(ctx, keep.Kind, drop.Name)
	require.ErrorIs(t, err, apperr.ErrIntegrationNotFound)

	survivor, err := store.GetByKindName(ctx, keep.Kind, keep.Name)
	require.NoError(t, err, "deleting one instance must not take its siblings with it")
	require.Equal(t, keep.Name, survivor.Name)
}

// Not-found is an error rather than a silent success: a delete that reports OK
// for a row that was never there hides a typo in the name, and the operator
// walks away believing a provider is gone.
func TestStore_DeleteMissingRowIsNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	setting := newSetting(t, seedDEK(ctx, t))
	_, err := store.Create(ctx, setting)
	require.NoError(t, err)

	require.ErrorIs(t, store.Delete(ctx, setting.Kind, "never-existed"), apperr.ErrIntegrationNotFound)
	require.ErrorIs(t, store.Delete(ctx, "no-such-kind", setting.Name), apperr.ErrIntegrationNotFound)
}

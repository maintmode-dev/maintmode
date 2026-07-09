package datakey

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
)

func TestStore_UpdateWrap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kr := keyringWith(t, "local-kms://kek-1", "local-kms://kek-1")
	stored, _ := seedDEK(ctx, t, kr, "update")

	// Re-wrap under a KEK the keyring still knows, so the row remains a valid,
	// unwrappable envelope after the update (rotation tests scan every row).
	dek, err := secrets.GenerateDEK()
	require.NoError(t, err)
	newWrapped, newKEKID, err := kr.WrapDEK(dek)
	require.NoError(t, err)
	require.NoError(t, store.UpdateWrap(ctx, stored.ID, newWrapped, newKEKID))

	found := findByID(ctx, t, stored.ID)
	require.Equal(t, newWrapped, found.EncryptedDEK)
	require.Equal(t, newKEKID, found.KEKID)
}

func TestStore_UpdateWrapMissingRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	err := store.UpdateWrap(ctx, uuid.New(), []byte{0x01}, "local-kms://kek-1")
	require.ErrorIs(t, err, apperr.ErrDataKeyNotFound)
}

// findByID locates a stored DEK by id via ListForUpdate. The store intentionally
// has no GetByID yet (rotation only needs list+update); tests read through the
// list to assert persisted state.
func findByID(ctx context.Context, t *testing.T, id uuid.UUID) *entity.DataKey {
	t.Helper()

	all, err := store.ListForUpdate(ctx)
	require.NoError(t, err)
	for _, dk := range all {
		if dk.ID == id {
			return dk
		}
	}
	t.Fatalf("data key %s not found", id)
	return nil
}

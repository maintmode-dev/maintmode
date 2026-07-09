package datakey

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/apperr"
)

func TestStore_GetByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Unique per-run KEK URI: the suite runs on a shared DB (-count 2).
	kekURI := "local-kms://get-" + uuid.NewString()
	kr := keyringWith(t, kekURI, kekURI)
	seeded, _ := seedDEK(ctx, t, kr, "get-by-id")

	got, err := store.GetByID(ctx, seeded.ID)
	require.NoError(t, err)
	require.Equal(t, seeded.ID, got.ID)
	require.Equal(t, seeded.KEKID, got.KEKID, "kek_id must round-trip non-empty")
	require.Equal(t, seeded.EncryptedDEK, got.EncryptedDEK, "wrapped DEK must round-trip byte-for-byte")
	require.NotNil(t, got.Label)
	require.False(t, got.CreatedAt.IsZero())

	// The read-back envelope must still unwrap — the row is usable, not just present.
	_, err = kr.UnwrapDEK(got.EncryptedDEK, got.KEKID)
	require.NoError(t, err)
}

func TestStore_GetByIDNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := store.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, apperr.ErrDataKeyNotFound)
}

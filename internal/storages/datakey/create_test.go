package datakey

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
	"github.com/ruko1202/maintmode/internal/pkg/secrets"
)

func TestStore_CreateAssignsIDAndTimestamp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kr := keyringWith(t, "local-kms://kek-1", "local-kms://kek-1")

	stored, _ := seedDEK(ctx, t, kr, "create")

	require.NotEqual(t, uuid.Nil, stored.ID, "id is assigned by the DB (uuidv7)")
	require.False(t, stored.CreatedAt.IsZero(), "created_at is set by the DB")
	require.Equal(t, "local-kms://kek-1", stored.KEKID)
	require.NotEmpty(t, stored.EncryptedDEK)
}

func TestStore_CreateWithNilLabel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kr := keyringWith(t, "local-kms://kek-1", "local-kms://kek-1")

	// label is nullable; a nil label must round-trip as NULL, not "". Wrap a real
	// DEK so the row stays a valid envelope (the shared data_keys table is scanned
	// wholesale by rotation tests — no non-envelope junk may be left behind).
	dek, err := secrets.GenerateDEK()
	require.NoError(t, err)
	wrapped, kekID, err := kr.WrapDEK(dek)
	require.NoError(t, err)

	stored, err := store.Create(ctx, &entity.DataKey{
		KEKID:        kekID,
		EncryptedDEK: wrapped,
		Label:        nil,
	})
	require.NoError(t, err)
	require.Nil(t, stored.Label)

	found := findByID(ctx, t, stored.ID)
	require.Nil(t, found.Label, "NULL label must read back as nil, not empty string")
}

package maintenances

import (
	"context"
	"testing"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store := NewStore(db)

	t.Run("ErrNoRows", func(t *testing.T) {
		t.Parallel()

		dbMaint, err := store.Get(ctx, xuuid.New())
		require.EqualError(t, err, qrm.ErrNoRows.Error())
		require.Nil(t, dbMaint)
	})
}

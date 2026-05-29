package notifytargets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/utils/xuuid"
)

func TestListByMaint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStore(db)

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		got, err := store.ListByMaint(ctx, xuuid.New())
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStore_List(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dekID := seedDEK(ctx, t)
	created, err := store.Create(ctx, newSetting(t, dekID))
	require.NoError(t, err)

	all, err := store.List(ctx)
	require.NoError(t, err)

	found := false
	for _, s := range all {
		if s.ID == created.ID {
			found = true
		}
	}
	require.True(t, found, "created integration must appear in List")
}

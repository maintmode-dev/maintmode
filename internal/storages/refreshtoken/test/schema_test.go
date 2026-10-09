package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSchema_KeepsOnlyTheClientAddress pins what a refresh-token row records
// about its client: the address, and not the User-Agent, which lives in the
// audit trail only.
func TestSchema_KeepsOnlyTheClientAddress(t *testing.T) {
	t.Parallel()

	var columns []string
	err := db.SelectContext(context.Background(), &columns, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'refresh_tokens'`)
	require.NoError(t, err)

	require.Contains(t, columns, "client_ip")
	require.NotContains(t, columns, "user_agent")
}

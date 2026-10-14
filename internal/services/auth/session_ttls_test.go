package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionExpiresAt_RoundsDownToUTCSeconds(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 10, 14, 12, 0, 0, 999_999_000, time.FixedZone("UTC+3", 3*60*60))

	got := sessionExpiresAt(startedAt, 720*time.Hour)

	require.Equal(t, time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), got)
	require.Equal(t, time.UTC, got.Location())
}

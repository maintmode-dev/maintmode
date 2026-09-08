package integration

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// Returning the far end's own words is the reason this endpoint exists, so
// bounding that text must never be able to eat it. An SMTP server is free to
// answer in latin-1 or with outright garbage, and a byte it does not encode
// properly has to cost that byte -- not the diagnostic.
func TestTruncateError(t *testing.T) {
	t.Parallel()

	t.Run("a short string is returned unchanged", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "535 auth failed", truncateError("535 auth failed"))
	})

	t.Run("a long string is bounded and stays valid", func(t *testing.T) {
		t.Parallel()

		out := truncateError(strings.Repeat("я", 400))
		require.LessOrEqual(t, len(out), probeErrorLimit)
		require.True(t, utf8.ValidString(out), "a split rune would break the JSON body")
	})

	t.Run("a rune straddling the limit stays valid", func(t *testing.T) {
		t.Parallel()

		// The cut lands mid-rune; the half left behind must not reach the body
		// as a broken byte.
		out := truncateError(strings.Repeat("x", probeErrorLimit-1) + "é" + "tail")
		require.True(t, utf8.ValidString(out))
		require.Contains(t, out, strings.Repeat("x", probeErrorLimit-1))
	})

	t.Run("invalid bytes cost themselves, not the message", func(t *testing.T) {
		t.Parallel()

		// The bad bytes sit at the START, so an implementation that shortens
		// the string until the whole prefix parses would return almost nothing.
		out := truncateError("535 auth failed\xff\xfe" + strings.Repeat("x", 600))
		require.Contains(t, out, "535 auth failed")
		require.True(t, utf8.ValidString(out))
	})

	t.Run("an all-garbage answer still says something", func(t *testing.T) {
		t.Parallel()

		out := truncateError(strings.Repeat("\xff", 600))
		require.NotEmpty(t, out, "an unreadable answer must not become an empty one")
		require.True(t, utf8.ValidString(out))
	})
}

package xemail

import (
	"testing"

	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/stretchr/testify/require"
)

// TestNormalizeStripsInvisibleCharacters is the regression test for a real
// bypass, so it names the characters that produced it.
//
// The per-address rate limiter buckets by address. Before this normalizer it
// used strings.ToLower(strings.TrimSpace(...)), which leaves zero-width and
// format characters in place -- while is.EmailFormat, the only validator in
// front of it, accepts them. Each suffix therefore produced a DIFFERENT bucket
// for the same victim, handing an attacker unlimited budgets against one
// address and, on the unknown-address path, an unbounded write into two indexed
// audit columns.
//
// The validator's acceptance is asserted here rather than assumed: if a future
// version starts rejecting these, this test says so instead of silently
// guarding nothing.
func TestNormalizeStripsInvisibleCharacters(t *testing.T) {
	t.Parallel()

	const base = "victim@example.com"

	for name, suffix := range map[string]string{
		"zero-width space":          "\u200b",
		"zero-width no-break space": "\ufeff",
		"word joiner":               "\u2060",
		"non-breaking space":        "\u00a0",
		"ascii space":               " ",
		"tab":                       "\t",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, base, Normalize(base+suffix),
				"%s must not survive normalization, or it splits the victim's rate-limit bucket", name)
		})
	}
}

// TestNormalizeFoldsCase pins the other half: without it "A@x" and "a@x" are two
// buckets and the tier is bypassed by holding shift.
func TestNormalizeFoldsCase(t *testing.T) {
	t.Parallel()

	require.Equal(t, "user@example.com", Normalize("USER@Example.COM"))
}

// TestNormalizeLeavesACanonicalAddressAlone guards the direction that would
// break ordinary users: a normal address must pass through untouched and be
// accepted.
func TestNormalizeLeavesACanonicalAddressAlone(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{
		"user@example.com",
		"first.last+tag@sub.example.co.uk",
		"u@e.io",
	} {
		require.Equal(t, addr, Normalize(addr), "%q is an ordinary address and must pass through untouched", addr)
		require.NoError(t, is.EmailFormat.Validate(addr))
	}
}

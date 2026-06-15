package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeOriginalURI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty falls back", "", "/"},
		{"plain relative path", "/calendar", "/calendar"},
		{"nested path with query", "/maintenance/123?edit=1", "/maintenance/123?edit=1"},
		{"root", "/", "/"},

		// Open-redirect vectors: the smuggled origin is always stripped, only the
		// path survives (on our origin). Origin-only inputs collapse to "/".
		{"absolute http", "http://evil.com", "/"},
		{"absolute https keeps only path", "https://evil.com/x", "/x"},
		{"scheme-relative", "//evil.com", "/"},
		{"scheme-relative keeps only path", "//evil.com/phish", "/phish"},

		// Normalized rather than rejected — still safely on our origin.
		{"backslash escaped to our origin", "/\\evil.com", "/%5Cevil.com"},
		{"not rooted gets rooted", "calendar", "/calendar"},
		{"relative dot normalized", "../../evil", "/evil"},

		// A legit same-origin path that merely *contains* a URL in its query is
		// preserved (origin stays ours).
		{"url in query is fine", "/redirect?to=http://evil.com", "/redirect?to=http://evil.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, safeOriginalURI(tc.in))
		})
	}
}

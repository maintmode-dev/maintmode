package otp

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The code reaches the user by email; the session nonce never does. That split
// is the whole browser-binding control -- an attacker who talks a victim into
// reading out the code still lacks the nonce, because it was never sent
// anywhere the victim could read it from.
func TestRenderOTPEmail_CarriesCodeNotNonce(t *testing.T) {
	t.Parallel()

	const (
		code  = "481920"
		nonce = "R7xQpLm4vT8sKd2wYn6bHc0jZaEuFgIo1rSt3MvXyPk="
	)

	body, err := RenderOTPEmail(code, 5*time.Minute)
	require.NoError(t, err)

	require.Contains(t, body, code)
	require.NotContains(t, body, nonce)
	require.Contains(t, body, "5 minutes")
}

// The body is injected into the transport's branded frame as raw template.HTML,
// so it must come out of html/template escaped. A code is always digits, but the
// escaping is a property of the renderer, not of today's inputs.
func TestRenderOTPEmail_EscapesInterpolatedValues(t *testing.T) {
	t.Parallel()

	body, err := RenderOTPEmail(`<script>alert(1)</script>`, time.Minute)
	require.NoError(t, err)

	require.NotContains(t, body, "<script>")
	require.Contains(t, body, "&lt;script&gt;")
}

func TestExpiresInPhrase(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ttl  time.Duration
		want string
	}{
		"whole minutes":           {5 * time.Minute, "5 minutes"},
		"singular":                {time.Minute, "1 minute"},
		"rounds up a remainder":   {90 * time.Second, "2 minutes"},
		"floors below a minute":   {20 * time.Second, "1 minute"},
		"zero still reads sanely": {0, "1 minute"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, expiresInPhrase(tc.ttl))
		})
	}
}

// The copy must not tell a recipient this is their only outstanding code:
// requesting again supersedes it, and someone holding two emails needs to know
// which one still works.
func TestRenderOTPEmail_PointsAtTheNewestCode(t *testing.T) {
	t.Parallel()

	body, err := RenderOTPEmail("000042", 5*time.Minute)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(body), "newest")
}

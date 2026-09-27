package otp

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ruko1202/maintmode/internal/entity"
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

	_, body, err := RenderOTPEmail(entity.OTPPurposeSignIn, code, 5*time.Minute)
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

	_, body, err := RenderOTPEmail(entity.OTPPurposeSignIn, `<script>alert(1)</script>`, time.Minute)
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

	_, body, err := RenderOTPEmail(entity.OTPPurposeSignIn, "000042", 5*time.Minute)
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(body), "newest")
}

// A reset code must not arrive dressed as a sign-in code. The sign-in copy
// tells someone who did not ask to "safely ignore" it, which is the wrong
// advice when what somebody is attempting is to replace their password -- the
// recipient has to be told that is what the code is for.
func TestRenderOTPEmail_ResetCodeSaysItIsAReset(t *testing.T) {
	t.Parallel()

	subject, body, err := RenderOTPEmail(entity.OTPPurposePasswordReset, "481920", 5*time.Minute)
	require.NoError(t, err)

	require.Contains(t, strings.ToLower(subject), "password reset")
	require.Contains(t, body, "481920")
	require.Contains(t, strings.ToLower(body), "reset your password")
	require.Contains(t, strings.ToLower(body), "has not changed")
	require.NotContains(t, strings.ToLower(subject+body), "sign-in")
}

// Tasks queued before the purpose existed carry none, and all of them were
// sign-in codes, so an empty purpose must render exactly as one.
func TestRenderOTPEmail_EmptyPurposeIsSignIn(t *testing.T) {
	t.Parallel()

	legacySubject, legacyBody, err := RenderOTPEmail("", "481920", 5*time.Minute)
	require.NoError(t, err)

	subject, body, err := RenderOTPEmail(entity.OTPPurposeSignIn, "481920", 5*time.Minute)
	require.NoError(t, err)

	require.Equal(t, subject, legacySubject)
	require.Equal(t, body, legacyBody)
}

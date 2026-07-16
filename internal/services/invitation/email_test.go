package invitation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestInvitationEmailSubject pins the product name into the subject so a
// recipient (and spam filters) see what the mail is for.
func TestInvitationEmailSubject(t *testing.T) {
	t.Parallel()
	require.Equal(t, "You've been invited to MaintMode", invitationEmailSubject)
	require.NotContains(t, invitationEmailSubject, "!") // spam-filter hygiene: no exclamation marks
}

// TestRenderInvitationEmailHTML asserts the rendered HTML body: the accept link
// is interpolated safely, the expiry line reflects the TTL, and the corrected
// "weren't expecting" copy replaced the old "don't know who you are" line.
func TestRenderInvitationEmailHTML(t *testing.T) {
	t.Parallel()

	const link = "https://app.example.com/accept-invite?token=abc123"
	body, err := renderInvitationEmail(link, 7*24*time.Hour)
	require.NoError(t, err)

	require.Contains(t, body, `<a href="`+link+`">Accept your invitation</a>`)
	require.Contains(t, body, "<p>This link expires in 7 days.</p>")
	require.Contains(t, body, "If you weren't expecting this invitation, you can safely ignore this email.")

	// The buggy original copy must be gone.
	require.NotContains(t, body, "If you don't know who you are")

	// Minimal-email constraint (RUK-155): no styling, no images, no <style>.
	require.NotContains(t, body, "<style")
	require.NotContains(t, body, "<img")
	require.NotContains(t, body, "style=")
}

// TestExpiresInPhrase covers day derivation from the TTL, including rounding up
// a sub-day remainder, the one-day singular, and the floor for tiny TTLs.
func TestExpiresInPhrase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ttl  time.Duration
		want string
	}{
		{"seven whole days", 7 * 24 * time.Hour, "7 days"},
		{"one day singular", 24 * time.Hour, "1 day"},
		{"rounds up remainder", 25 * time.Hour, "2 days"},
		{"floors sub-day to one day", time.Hour, "1 day"},
		{"three days", 3 * 24 * time.Hour, "3 days"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, expiresInPhrase(tc.ttl))
		})
	}
}

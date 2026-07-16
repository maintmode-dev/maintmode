package emailtransport

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrapHTML_FramesBodyAndDegrades(t *testing.T) {
	t.Parallel()

	const link = "https://app.test/accept?token=abc"
	inner := `<p>You've been invited to join MaintMode.</p>` +
		`<p><a href="` + link + `">Accept your invitation</a></p>`

	out, err := wrapHTML(inner, "If you weren't expecting this, you can ignore this email.")
	require.NoError(t, err)

	// Frame is present: wordmark header, the inner body verbatim, the footer line.
	// The footer is plain text, so html/template escapes its apostrophe — assert
	// the escaped form here; the readable form is checked on the plain text below.
	require.Contains(t, out, ">MaintMode<")
	require.Contains(t, out, inner, "inner body must be injected raw, not re-escaped")
	require.Contains(t, out, "If you weren&#39;t expecting this, you can ignore this email.")

	// Email-client-safe constraints: table layout, inline styles only, no
	// <style> block, no images, no scripts, no forms, no webfonts/links.
	require.Contains(t, out, "<table")
	require.Contains(t, out, `max-width:600px`)
	require.NotContains(t, out, "<style")
	require.NotContains(t, out, "class=")
	require.NotContains(t, out, "<img")
	require.NotContains(t, out, "<script")
	require.NotContains(t, out, "<form")
	require.NotContains(t, out, "<link")

	// Plain-text degradation: header, body and footer land on separate, readable
	// lines; the link survives as "text (url)"; no stray tags or gibberish.
	text := htmlToText(out)
	require.Contains(t, text, "MaintMode")
	require.Contains(t, text, "You've been invited to join MaintMode.")
	require.Contains(t, text, "Accept your invitation ("+link+")")
	require.Contains(t, text, "If you weren't expecting this, you can ignore this email.")
	require.NotContains(t, text, "<")
	require.NotContains(t, text, ">")
	// No tall gaps: the spacer styling must not leak blank-line runs.
	require.NotContains(t, text, "\n\n\n")
}

// TestWrapHTML_InvitationEmailEndToEnd feeds the invitation email's inner body
// (as the invitation service renders it) through the frame and the plain-text
// derivation, pinning the full shape a recipient sees. The inner markup mirrors
// internal/services/invitation/templates/invitation_email.gohtml; if that
// template changes, update this fixture.
func TestWrapHTML_InvitationEmailEndToEnd(t *testing.T) {
	t.Parallel()

	const link = "https://app.maintmode.dev/accept-invite?token=Xa9F2b"
	inner := `<p>You've been invited to join MaintMode.</p>` + "\n" +
		`<p>MaintMode is a new way to manage your maintenance status.</p>` + "\n" +
		`<p><a href="` + link + `">Accept your invitation</a></p>` + "\n" +
		`<p>This link expires in 7 days.</p>` + "\n" +
		`<p>If you weren't expecting this invitation, you can safely ignore this email.</p>`

	out, err := wrapHTML(inner, "")
	require.NoError(t, err)

	text := htmlToText(out)
	require.Contains(t, text, "MaintMode\n")
	require.Contains(t, text, "You've been invited to join MaintMode.")
	require.Contains(t, text, "Accept your invitation ("+link+")")
	require.Contains(t, text, "This link expires in 7 days.")
	require.Contains(t, text, "If you weren't expecting this invitation, you can safely ignore this email.")
	require.Contains(t, text, defaultFooterText)
	require.NotContains(t, text, "<")
	require.NotContains(t, text, "\n\n\n")
}

func TestWrapHTML_EmptyFooterFallsBackToDefault(t *testing.T) {
	t.Parallel()

	out, err := wrapHTML(`<p>hi</p>`, "   ")
	require.NoError(t, err)
	require.Contains(t, out, defaultFooterText)
}

func TestWrapHTML_FooterIsEscaped(t *testing.T) {
	t.Parallel()

	// Footer is plain text and must be auto-escaped by html/template so a stray
	// angle bracket can't inject markup into the frame.
	out, err := wrapHTML(`<p>hi</p>`, "a < b & c")
	require.NoError(t, err)
	require.Contains(t, out, "a &lt; b &amp; c")
	require.NotContains(t, out, "a < b & c")
}

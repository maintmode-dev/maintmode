package emailtransport

import (
	"html/template"
	"strings"

	"github.com/ruko1202/maintmode/internal/gateways/notifytransport/email/templates"
)

// Branded email layout.
//
// Every HTML email MaintMode sends is wrapped in one shared frame so messages
// look like the product rather than bare markup, without each service knowing
// about the layout — Send applies it centrally (see Send, HTML path only). The
// markup lives in templates/layout.gohtml, embedded and parsed once here.
//
// The markup is deliberately conservative for email clients: a table-based
// layout (Outlook has no real CSS box model), a ~600px centered card, and every
// style inline in a style="" attribute — no <style> block, no classes, no
// external CSS, no webfonts, no images, no JS. The product name is a text
// wordmark, not a logo image (remote images are blocked by default in most
// clients and hurt spam scoring). There is no colored accent and no CTA button
// by design: the frame stays neutral and text-dominant, which keeps spam
// scoring clean; a branded accent is deferred until real brand tokens exist.
// Colors and the font stack are inlined in the template; they are placeholder
// brand tokens, to be swapped when real ones exist.
//
// Plain-text degradation is a hard requirement (the transport derives the
// text/plain alternative from this HTML via htmlToText). The only text the frame
// itself adds is the wordmark and the footer line; all spacing is CSS padding,
// which carries no text and so leaves no trace in the plain-text version.
// htmlToText maps </tr> to a newline, so the header, content and footer rows land
// on separate lines, and it collapses the resulting run of blank lines (a body
// ending in </p> → "\n\n" immediately followed by the row's </tr> → "\n" would
// otherwise stack to three) down to a single blank line.
const layoutTemplate = "layout.gohtml"

// defaultFooterText is used when a caller does not supply a message-specific
// footer line. It states the product and why the recipient received the mail.
const defaultFooterText = "You received this email because of your MaintMode account activity."

// layoutTmpl renders the branded frame around an inner HTML body, parsed once
// from the embedded templates. Body is injected as template.HTML (raw): it is
// already-rendered, html/template-escaped content produced by the sending
// service, not untrusted input, so re-escaping it here would corrupt the markup.
// Footer is plain text and auto-escaped.
var layoutTmpl = template.Must(
	template.ParseFS(templates.FS, layoutTemplate),
)

type layoutData struct {
	Body   template.HTML
	Footer string
}

// wrapHTML wraps an already-rendered inner HTML body in the branded frame.
// footer is the muted footer line; an empty footer falls back to
// defaultFooterText. The returned string is the full HTML email body.
//
// SECURITY INVARIANT: innerBody is injected raw (template.HTML), so it MUST be
// trusted markup — html/template output rendered by the sending service, never a
// string built from user input. This holds because wrapHTML runs only on the
// HTMLMessageMIME path (see Send), and the producers of an HTML NotifyMessage
// today are the invitation email and the one-time sign-in code email. Both
// interpolate only server-authored fields — an accept link and a derived expiry
// phrase for the invitation, a generated six-digit code and an expiry phrase for
// the code — and no recipient/inviter data. The untrusted-content path
// (maintenance notifications, which carry a user-supplied title) renders via
// text/template as TextMessageMIME and bypasses this function entirely. Any future HTML producer must likewise render through
// html/template before its body reaches here, or this becomes an HTML-injection
// sink. The footer, by contrast, is a plain string and html/template auto-escapes
// it, so it is safe even if a caller ever derives it from untrusted data.
func wrapHTML(innerBody, footer string) (string, error) {
	if strings.TrimSpace(footer) == "" {
		footer = defaultFooterText
	}

	var buf strings.Builder
	if err := layoutTmpl.Execute(&buf, layoutData{
		Body:   template.HTML(innerBody), //nolint:gosec // trusted service-rendered html/template output; see SECURITY INVARIANT above.
		Footer: footer,
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

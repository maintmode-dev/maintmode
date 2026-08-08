package emailtransport

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
	"github.com/wneessen/go-mail"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Send delivers msg to target (an email address). msg.Body is treated as HTML;
// a plain-text alternative is attached so clients without HTML rendering still
// show readable content.
//
// Threads are not supported by design: e-mail has no addressable message this
// system could reply to later, so replyTo is ignored and the returned
// SendResult is empty. A nil Ref means callers never record a root for this
// transport, which is the intended behavior, not a missing feature.
func (c *Client) Send(
	ctx context.Context,
	target string,
	msg entity.NotifyMessage,
	_ *entity.MessageRef,
) (entity.SendResult, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "email.Send")
	defer span.End()

	m := mail.NewMsg()
	if err := m.From(c.from); err != nil {
		return entity.SendResult{}, fmt.Errorf("email from %q: %w", c.from, err)
	}
	if err := m.To(target); err != nil {
		return entity.SendResult{}, fmt.Errorf("email to %q: %w", target, err)
	}
	if c.replyTo != "" {
		if err := m.ReplyTo(c.replyTo); err != nil {
			return entity.SendResult{}, fmt.Errorf("email reply-to %q: %w", c.replyTo, err)
		}
	}

	body := msg.Body
	if msg.MessageMIME == entity.HTMLMessageMIME {
		// Wrap HTML bodies in the shared branded frame (see layout.go). Plain-text
		// messages pass through untouched — there is nothing to frame, and the
		// wordmark/footer would just be noise in an already-plain body.
		wrapped, err := wrapHTML(msg.Body, "")
		if err != nil {
			return entity.SendResult{}, fmt.Errorf("wrap email body: %w", err)
		}
		body = wrapped
	}

	m.Subject(msg.Subject)
	m.SetBodyString(msgMIME(msg.MessageMIME), body)
	// The text/plain alternative is derived from the final (wrapped) HTML so it
	// matches what an HTML client renders, frame included.
	m.AddAlternativeString(mail.TypeTextPlain, htmlToText(body))

	if err := c.client.DialAndSendWithContext(ctx, m); err != nil {
		xlog.Error(ctx, "email send", xfield.Error(err))
		return entity.SendResult{}, fmt.Errorf("email send: %w", err)
	}
	return entity.SendResult{}, nil
}

func msgMIME(mime entity.MessageMIME) mail.ContentType {
	switch mime {
	case entity.HTMLMessageMIME:
		return mail.TypeTextHTML
	case entity.TextMessageMIME:
		return mail.TypeTextPlain
	default:
		return mail.TypeTextPlain
	}
}

var (
	tagRE = regexp.MustCompile(`<[^>]+>`)
	// anchorRE captures an <a href="URL">TEXT</a> so the plain-text fallback can
	// keep the URL — otherwise stripping tags would drop the link, which for an
	// invitation email is the whole point.
	anchorRE = regexp.MustCompile(`(?is)<a\b[^>]*\bhref="([^"]*)"[^>]*>(.*?)</a>`)
	// blankLinesRE collapses any run of 3+ newlines to a single blank line. The
	// branded layout nests tables (rows inside a card inside a page cell), so
	// several block-close newlines can pile up between the header, body and
	// footer; without this the plain-text fallback grows tall gaps.
	blankLinesRE = regexp.MustCompile(`\n{3,}`)
)

// htmlToText produces a best-effort plain-text fallback for the HTML body: it
// expands anchors to "text (url)", turns block breaks into newlines, strips the
// remaining tags, and unescapes HTML entities (the body comes from html/template,
// which escapes `&`/`<`/`>`). It is not a full HTML renderer — just enough so a
// non-HTML client still sees readable content and any links.
//
// Table rows are treated as block breaks (</tr> → newline) so the branded email
// layout — a table whose rows are the header, content and footer — degrades into
// separated lines rather than one run-on line. Cell breaks (</td>) are left to
// strip to nothing: both the frame and every HTML body that flows through it hold
// a single content cell per row, so a row-level break is the right granularity
// and a cell-level one would only double up. A future HTML body that puts
// side-by-side cells in one row would collapse to "LeftRight" here and must add
// </td> → " " (or similar) at that point.
func htmlToText(body string) string {
	text := anchorRE.ReplaceAllString(body, "$2 ($1)")
	replacer := strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
		"</p>", "\n\n", "</div>", "\n", "</tr>", "\n",
	)
	text = replacer.Replace(text)
	text = tagRE.ReplaceAllString(text, "")
	text = blankLinesRE.ReplaceAllString(text, "\n\n")
	return html.UnescapeString(strings.TrimSpace(text))
}

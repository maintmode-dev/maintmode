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
func (c *Client) Send(ctx context.Context, target string, msg entity.NotifyMessage) error {
	ctx, span := xlog.WithOperationSpan(ctx, "email.Send")
	defer span.End()

	m := mail.NewMsg()
	if err := m.From(c.from); err != nil {
		return fmt.Errorf("email from %q: %w", c.from, err)
	}
	if err := m.To(target); err != nil {
		return fmt.Errorf("email to %q: %w", target, err)
	}
	if c.replyTo != "" {
		if err := m.ReplyTo(c.replyTo); err != nil {
			return fmt.Errorf("email reply-to %q: %w", c.replyTo, err)
		}
	}

	m.Subject(msg.Subject)
	m.SetBodyString(msgMIME(msg.MessageMIME), msg.Body)
	m.AddAlternativeString(mail.TypeTextPlain, htmlToText(msg.Body))

	if err := c.client.DialAndSendWithContext(ctx, m); err != nil {
		xlog.Error(ctx, "email send", xfield.Error(err))
		return fmt.Errorf("email send: %w", err)
	}
	return nil
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
)

// htmlToText produces a best-effort plain-text fallback for the HTML body: it
// expands anchors to "text (url)", turns block breaks into newlines, strips the
// remaining tags, and unescapes HTML entities (the body comes from html/template,
// which escapes `&`/`<`/`>`). It is not a full HTML renderer — just enough so a
// non-HTML client still sees readable content and any links.
func htmlToText(body string) string {
	text := anchorRE.ReplaceAllString(body, "$2 ($1)")
	replacer := strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
		"</p>", "\n\n", "</div>", "\n",
	)
	text = replacer.Replace(text)
	text = tagRE.ReplaceAllString(text, "")
	return html.UnescapeString(strings.TrimSpace(text))
}
